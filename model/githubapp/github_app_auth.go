package githubapp

import (
	"context"
	"time"

	"github.com/evergreen-ci/evergreen"
	"github.com/google/go-github/v70/github"
	"github.com/pkg/errors"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const (
	githubAppEndpointAttribute   = "evergreen.githubapp.endpoint"
	githubAppAttemptAttribute    = "evergreen.githubapp.attempt"
	githubAppURLAttribute        = "evergreen.githubapp.url"
	githubAppErrorAttribute      = "evergreen.githubapp.error"
	githubAppMethodAttribute     = "evergreen.githubapp.method"
	githubAppStatusCodeAttribute = "evergreen.githubapp.status_code"
)

// GithubAppAuth holds the appId and privateKey for the github app associated with the project.
// It will not be stored along with the project settings, instead it is fetched only when needed
// Sometimes this struct is used as a way to pass around AppId and PrivateKey for Evergreen's
// github app, in which the Id is set to empty.
type GithubAppAuth struct {
	// Should match the ID of the project it refers to
	Id string `bson:"_id" json:"_id"`

	// AppID is the GitHub app's ID.
	AppID int64 `bson:"app_id" json:"app_id"`
	// PrivateKey is the GitHub app's private key and is intentionally not
	// stored in the database for security reasons. The private key can be
	// fetched from Parameter Store using PrivateKeyParameter.
	PrivateKey []byte `bson:"-" json:"private_key"`
	// PrivateKeyParameter is the name of the parameter that holds the
	// GitHub app's private key.
	PrivateKeyParameter string `bson:"private_key_parameter" json:"private_key_parameter"`
}

// CreateGitHubAppAuth returns the Evergreen-internal app id and app private key
// if they exist. If the either are not set, it will return nil.
func CreateGitHubAppAuth(settings *evergreen.Settings) *GithubAppAuth {
	if settings.AuthConfig.Github == nil || settings.AuthConfig.Github.AppId == 0 {
		settings = evergreen.GetEnvironment().Settings()
		if settings.AuthConfig.Github == nil || settings.AuthConfig.Github.AppId == 0 {
			return nil
		}
	}

	key := settings.Expansions[evergreen.GithubAppPrivateKey]
	if key == "" {
		return nil
	}

	return &GithubAppAuth{
		AppID:      settings.AuthConfig.Github.AppId,
		PrivateKey: []byte(key),
	}
}

// IsGithubAppInstalledOnRepo returns true if the GitHub app is installed on given owner/repo.
func (g *GithubAppAuth) IsGithubAppInstalledOnRepo(ctx context.Context, owner, repo string) (bool, error) {
	if g == nil || g.AppID == 0 || len(g.PrivateKey) == 0 {
		return false, errors.New("no github app auth provided")
	}

	installationID, err := getInstallationID(ctx, g, owner, repo)
	if err != nil {
		return false, errors.Wrapf(err, "getting installation id for '%s/%s'", owner, repo)
	}

	return installationID != 0, nil
}

// CreateCachedInstallationTokenWithDefaultOwnerRepo is the same as
// CreateCachedInstallationToken but specifically returns an installation token
// from a default owner/repo. This is useful for scenarios when we do not care
// about the owner/repo that we are calling the GitHub function with (i.e.
// checking rate limit). It will use the default owner/repo specified in the
// admin settings and error if it's not set.
func CreateCachedInstallationTokenWithDefaultOwnerRepo(ctx context.Context, settings *evergreen.Settings, lifetime time.Duration, opts *github.InstallationTokenOptions) (string, error) {
	if settings.AuthConfig.Github == nil || settings.AuthConfig.Github.DefaultOwner == "" || settings.AuthConfig.Github.DefaultRepo == "" {
		return "", errors.Errorf("missing GitHub app configuration needed to create installation tokens")
	}
	return CreateGitHubAppAuth(settings).CreateCachedInstallationToken(ctx, settings.AuthConfig.Github.DefaultOwner, settings.AuthConfig.Github.DefaultRepo, lifetime, opts)
}

// CreateCachedInstallationToken uses the owner/repo information to request an github app installation id
// and uses that id to create an installation token.
// If possible, it will try to use an existing installation token for the app
// from the cache, unless that cached token will expire before the requested
// lifetime. For example, if requesting a token that should be valid for the
// next 30 minutes, this method can return a cached token that is still valid
// for 45 minutes. However, if the cached token will expire in 5 minutes, it
// will provide a freshly-generated token. Also take special care if revoking a
// token returned from this method - revoking the token will cause other GitHub
// operations reusing the same token to fail.
func (g *GithubAppAuth) CreateCachedInstallationToken(ctx context.Context, owner, repo string, lifetime time.Duration, opts *github.InstallationTokenOptions) (string, error) {
	if lifetime >= MaxInstallationTokenLifetime {
		lifetime = MaxInstallationTokenLifetime
	}

	if g == nil {
		return "", errors.New("GitHub app is not configured in admin settings")
	}

	installationID, err := getInstallationID(ctx, g, owner, repo)
	if err != nil {
		return "", errors.Wrapf(err, "getting installation id for '%s/%s'", owner, repo)
	}

	id, err := createCacheID(installationID, opts.GetPermissions())
	if err != nil {
		return "", errors.Wrap(err, "creating cache ID")
	}
	if cachedToken, found := ghInstallationTokenCache.Get(ctx, id, lifetime); found {
		return cachedToken, nil
	}

	createdAt := time.Now()
	token, _, err := g.createInstallationTokenForID(ctx, installationID, opts)
	if err != nil {
		return "", errors.Wrap(err, "creating installation token")
	}

	ghInstallationTokenCache.Put(ctx, id, token, createdAt.Add(MaxInstallationTokenLifetime))

	return token, errors.Wrapf(err, "getting installation token for '%s/%s'", owner, repo)
}

// CreateCachedInstallationTokenForGitHubSender is a helper that creates a
// cached installation token for the given owner/repo for the GitHub sender.
func (g *GithubAppAuth) CreateGitHubSenderInstallationToken(ctx context.Context, owner, repo string) (string, error) {
	return g.CreateCachedInstallationToken(ctx, owner, repo, MaxInstallationTokenLifetime, nil)
}

// CreateInstallationToken creates an installation token for the given
// owner/repo. This is never cached, and should only be used in scenarios where
// the token can be revoked at any time.
func (g *GithubAppAuth) CreateInstallationToken(ctx context.Context, owner, repo string, opts *github.InstallationTokenOptions) (string, *github.InstallationPermissions, error) {
	installationID, err := getInstallationID(ctx, g, owner, repo)
	if err != nil {
		return "", nil, errors.Wrapf(err, "getting installation id for '%s/%s'", owner, repo)
	}

	token, permissions, err := g.createInstallationTokenForID(ctx, installationID, opts)
	if err != nil {
		return "", nil, errors.Wrapf(err, "creating installation token for '%s/%s'", owner, repo)
	}

	return token, permissions, nil
}

// createInstallationTokenForID returns an installation token from GitHub given an installation ID.
// This function cannot be moved to thirdparty because it is needed to set up the environment.
func (g *GithubAppAuth) createInstallationTokenForID(ctx context.Context, installationID int64, opts *github.InstallationTokenOptions) (string, *github.InstallationPermissions, error) {
	const caller = "CreateInstallationToken"
	ctx, span := tracer.Start(ctx, caller, trace.WithAttributes(
		attribute.String(githubAppEndpointAttribute, caller),
	))
	defer span.End()

	client, err := getGitHubClientForAuth(g)
	if err != nil {
		return "", nil, errors.Wrap(err, "getting GitHub client for token creation")
	}
	defer client.Close()

	token, resp, err := client.Apps.CreateInstallationToken(ctx, installationID, opts)
	if resp != nil {
		defer resp.Body.Close()
		span.SetAttributes(attribute.Int(githubAppStatusCodeAttribute, resp.StatusCode))
	}
	if err != nil {
		span.SetAttributes(attribute.String(githubAppErrorAttribute, err.Error()))
		return "", nil, errors.Wrapf(err, "creating installation token for installation id: %d", installationID)
	}
	if token == nil {
		err := errors.Errorf("Installation token for installation 'id': %d not found", installationID)
		span.SetAttributes(attribute.String(githubAppErrorAttribute, err.Error()))
		return "", nil, err
	}

	return token.GetToken(), token.GetPermissions(), nil
}

// RedactPrivateKey redacts the GitHub app's private key so that it's not exposed via the UI or GraphQL.
func (g *GithubAppAuth) RedactPrivateKey() *GithubAppAuth {
	g.PrivateKey = []byte(evergreen.RedactedValue)
	return g
}

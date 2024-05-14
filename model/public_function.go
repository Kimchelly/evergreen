package model

import (
	"context"
	"fmt"
	"strings"

	"github.com/evergreen-ci/evergreen"
	"github.com/mongodb/anser/bsonutil"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
)

const Collection = "public_funcs"

// PublicFunction are user-defined Evergreen functions that can be shared across
// projects.
// kim: NOTE: this only allows a raw list of commands. Does not support calling
// other funcs (which aren't defined) or other public funcs (which could result
// in complexity/weirdness/too much fn calls).
// kim: NOTE: unlike private funcs, can only access the vars you explicitly pass
// in to avoid passing in secrets or conflicting expansions unintentionally.
type PublicFunction struct {
	// Unique identifier to support multiple versions of the same func.
	ID              string `bson:"_id" json:"id"`
	FunctionVersion `bson:",inline"`
	// User-friendly description to help understand how to use it.
	Description string
	Commands    YAMLCommandSet `bson:"commands" json:"commands"`
	// Could support validation of user-defined required input vars, but why
	// bother now.
}

var (
	FunctionNameKey    = bsonutil.MustHaveTag(PublicFunction{}, "Name")
	FunctionVersionKey = bsonutil.MustHaveTag(PublicFunction{}, "Version")
)

// One version of a public function, e.g. send-slack-notification@v1.2.3
type FunctionVersion struct {
	// Name of the function. Can't reuse existing command names (e.g.
	// shell.exec), or function names already defined in the YAML.
	Name string `bson:"name" json:"name"`
	// Semver just in case the user implementation changes in the future.
	// kim: NOTE: only supports explicit version strings for now, not implicit
	// "latest", because I don't feel like writing the aggregation to get the
	// max value.
	Version string `bson:"version" json:"version"`
}

func (fv FunctionVersion) String() string {
	return fmt.Sprintf("%s@%s", fv.Name, fv.Version)
}

func NewFunctionVersion(name, version string) FunctionVersion {
	return FunctionVersion{
		Name:    name,
		Version: version,
	}
}

// NewFunctionVersionFromString parses a string in the format "name@version"
// (e.g. "send-slack-notification@v1.2.3") into a FunctionVersion.
func NewFunctionVersionFromString(nameAndVersion string) FunctionVersion {
	parts := strings.Split(nameAndVersion, "@")
	if len(parts) != 2 {
		return FunctionVersion{}
	}
	return NewFunctionVersion(parts[0], parts[1])
}

func MakePublicFunctionMap(pubFuncs []PublicFunction) map[FunctionVersion]PublicFunction {
	m := map[FunctionVersion]PublicFunction{}
	for _, f := range pubFuncs {
		m[f.FunctionVersion] = f
	}
	grip.Info(message.Fields{
		"message": "kim: made public functions map",
		"map":     fmt.Sprintf("%#v", m),
	})
	return m
}

func FindPublicFunctionsByFunctionVersions(ctx context.Context, funcVers ...FunctionVersion) ([]PublicFunction, error) {
	if len(funcVers) == 0 {
		return nil, nil
	}

	matchNameAndVersion := []bson.M{}
	for _, fv := range funcVers {
		matchNameAndVersion = append(matchNameAndVersion, bson.M{
			FunctionNameKey: fv.Name,
			// kim: NOTE: need to handle latest (i.e. either "@latest" or just
			// the function name without an "@latest")
			FunctionVersionKey: fv.Version,
		})
	}
	q := bson.M{
		"$or": matchNameAndVersion,
	}
	res, err := evergreen.GetEnvironment().DB().Collection(Collection).Find(ctx, q)
	if err != nil {
		return nil, errors.Wrap(err, "finding public functions by function version")
	}
	pubFuncs := []PublicFunction{}
	if err := res.All(ctx, &pubFuncs); err != nil {
		return nil, errors.Wrap(err, "decoding public functions")
	}

	if err := resolveParams(pubFuncs); err != nil {
		return nil, err
	}

	return pubFuncs, nil
}

func resolveParams(pubFuncs []PublicFunction) error {
	catcher := grip.NewBasicCatcher()
	for _, pubFunc := range pubFuncs {
		catcher.Wrapf(pubFunc.resolveParams(), "resolving params for public function '%s'", pubFunc.FunctionVersion.String())
	}
	return catcher.Resolve()
}

func (pf *PublicFunction) resolveParams() error {
	cmds := pf.Commands.List()
	for idx := range cmds {
		// Projects (and therefore public funcs) do this weird YAML dance where
		// it stores the params as a YAML string, then sets the params from the
		// string when marshalling the YAML.
		if err := cmds[idx].resolveParams(); err != nil {
			return errors.Wrapf(err, "resolving params for command #%d", idx+1)
		}
	}
	return nil
}

// kim: NOTE: returns all versions across all public function names.
func FindAllPublicFunctions(ctx context.Context, q bson.M) ([]PublicFunction, error) {
	pubFuncs := []PublicFunction{}
	cur, err := evergreen.GetEnvironment().DB().Collection(Collection).Find(ctx, q)
	if err := cur.All(ctx, &pubFuncs); err != nil {
		return nil, err
	}

	if err := resolveParams(pubFuncs); err != nil {
		return nil, err
	}

	return pubFuncs, err
}

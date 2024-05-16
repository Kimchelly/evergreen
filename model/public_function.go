package model

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/evergreen-ci/evergreen"
	"github.com/mongodb/anser/bsonutil"
	"github.com/mongodb/grip"
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
	// Versioned functions in case the user implementation changes in the
	// future. These are just ints, but it can be the special string "latest" if
	// looking to use the latest version of a public function.
	Version string `bson:"version" json:"version"`
}

func (fv FunctionVersion) String() string {
	return fmt.Sprintf("%s@%s", fv.Name, fv.Version)
}

func (fv FunctionVersion) IsLatest() bool {
	return fv.Version == "" || fv.Version == LatestPublicFunctionVersion
}

func NewFunctionVersion(name, version string) FunctionVersion {
	return FunctionVersion{
		Name:    name,
		Version: version,
	}
}

// NewFunctionVersionFromString parses a string in the format "name@version"
// (e.g. "send-slack-notification@v1") into a FunctionVersion. If no version
// is specified, it's assumed to be the latest.
func NewFunctionVersionFromString(nameAndVersion string) FunctionVersion {
	parts := strings.Split(nameAndVersion, "@")
	if len(parts) != 2 {
		// If there's no "@version" part, use the latest version.
		return FunctionVersion{
			Name:    nameAndVersion,
			Version: LatestPublicFunctionVersion,
		}
	}
	name := parts[0]
	version := strings.TrimPrefix(parts[1], "v")
	return NewFunctionVersion(name, version)
}

type SortablePublicFunctions []PublicFunction

func (spf SortablePublicFunctions) Len() int { return len(spf) }
func (spf SortablePublicFunctions) Swap(i, j int) {
	spf[i], spf[j] = spf[j], spf[i]
}

func (spf SortablePublicFunctions) Less(i, j int) bool {
	pubFunc1 := spf[i]
	pubFunc2 := spf[j]
	if pubFunc1.Name != pubFunc2.Name {
		return pubFunc1.Name < pubFunc2.Name
	}
	return pubFunc1.Version < pubFunc2.Version
}

// Special constant to indicate that the current latest version of a function
// should be used.
const LatestPublicFunctionVersion = "latest"

func (spf SortablePublicFunctions) Find(fv FunctionVersion) *PublicFunction {
	if len(spf) == 0 {
		return nil
	}

	if fv.IsLatest() {
		if !sort.IsSorted(spf) {
			sort.Sort(spf)
		}
		// Since it's sorted in version ascending order, the last one is the
		// latest version.
		return &spf[len(spf)-1]
	}

	for _, pf := range spf {
		if pf.FunctionVersion == fv {
			return &pf
		}
	}
	return nil
}

// MakeSortedPublicFunctionMap returns a map of public function name to a sorted
// list of public functions, one per version of that function. For each
// function name, the versions of it are sorted in ascending order.
func MakeSortedPublicFunctionMap(pubFuncs []PublicFunction) map[string]SortablePublicFunctions {
	m := map[string]SortablePublicFunctions{}
	for _, pubFunc := range pubFuncs {
		m[pubFunc.Name] = append(m[pubFunc.Name], pubFunc)
	}
	for name, pubFuncs := range m {
		sortedPubFuncs := pubFuncs
		sort.Sort(sortedPubFuncs)
		m[name] = sortedPubFuncs
	}
	return m
}

// FindPublicFunctionsByFunctionVersions returns all public functions that match
// the given function versions. Public functions are returned in no particular
// order.
func FindPublicFunctionsByFunctionVersions(ctx context.Context, funcVers ...FunctionVersion) ([]PublicFunction, error) {
	if len(funcVers) == 0 {
		return nil, nil
	}

	matchNameAndVersion := []bson.M{}
	matchNameAndLatestVersion := map[string]struct{}{}
	for _, fv := range funcVers {
		if fv.IsLatest() {
			matchNameAndLatestVersion[fv.Name] = struct{}{}
			continue
		}

		matchNameAndVersion = append(matchNameAndVersion, bson.M{
			FunctionNameKey:    fv.Name,
			FunctionVersionKey: fv.Version,
		})
	}

	explicitVersionPubFuncs := []PublicFunction{}
	if len(matchNameAndVersion) > 0 {
		// Get public functions with explicit versions.
		q := bson.M{
			"$or": matchNameAndVersion,
		}
		res, err := evergreen.GetEnvironment().DB().Collection(Collection).Find(ctx, q)
		if err != nil {
			return nil, errors.Wrap(err, "finding public functions by function version")
		}
		if err := res.All(ctx, &explicitVersionPubFuncs); err != nil {
			return nil, errors.Wrap(err, "decoding public functions")
		}
	}

	latestVersionPubFuncs := []PublicFunction{}
	if len(matchNameAndLatestVersion) > 0 {
		// Get latest versions of public functions.
		latestVersionNames := make([]string, 0, len(matchNameAndLatestVersion))
		for name := range matchNameAndLatestVersion {
			latestVersionNames = append(latestVersionNames, name)
		}
		// Source: https://www.mongodb.com/community/forums/t/selecting-documents-with-largest-value-of-a-field/107032
		cur, err := evergreen.GetEnvironment().DB().Collection(Collection).Aggregate(ctx, []bson.M{
			{"$match": bson.M{FunctionNameKey: bson.M{"$in": latestVersionNames}}},
			{"$sort": bson.M{FunctionNameKey: 1, FunctionVersionKey: -1}},
			{"$group": bson.M{"_id": "$name", "doc_with_latest_version": bson.M{"$first": "$$ROOT"}}},
			{"$replaceWith": "$doc_with_latest_version"},
		})
		if err != nil {
			return nil, errors.Wrapf(err, "finding latest versions of public functions %s", latestVersionNames)
		}
		if err := cur.All(ctx, &latestVersionPubFuncs); err != nil {
			return nil, errors.Wrapf(err, "decoding latest versions of public functions '%s'", latestVersionNames)
		}
	}

	pubFuncs := append(explicitVersionPubFuncs, latestVersionPubFuncs...)

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

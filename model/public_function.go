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
	// future. These are just ints, but it can be the special string "latest".
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

	if fv.Version == "" || fv.Version == LatestPublicFunctionVersion {
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

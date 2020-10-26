package pool

import (
	"github.com/evergreen-ci/evergreen/db"
	"github.com/mongodb/anser/bsonutil"
	adb "github.com/mongodb/anser/db"
	"go.mongodb.org/mongo-driver/bson"
)

const (
	Collection = "bonusly_pools"
)

var (
	IDKey         = bsonutil.MustHaveTag(BettingPool{}, "ID")
	TaskIDKey     = bsonutil.MustHaveTag(BettingPool{}, "TaskID")
	VersionIDKey  = bsonutil.MustHaveTag(BettingPool{}, "VersionID")
	BetIDsKey     = bsonutil.MustHaveTag(BettingPool{}, "BetIDs")
	MinimumBetKey = bsonutil.MustHaveTag(BettingPool{}, "MinimumBet")
)

func ByID(id string) db.Q {
	return db.Query(bson.M{
		IDKey: id,
	})
}

// FindOne finds a single betting pool that satisfies the query.
func FindOne(query db.Q) (*BettingPool, error) {
	var bp BettingPool
	err := db.FindOneQ(Collection, query, &bp)
	if adb.ResultsNotFound(err) {
		return nil, nil
	}
	return &bp, err
}

// FindAlls returns all betting pools that satisfy the query.
func FindAll(query db.Q) ([]BettingPool, error) {
	var bps []BettingPool
	err := db.FindAllQ(Collection, query, &bps)
	if adb.ResultsNotFound(err) {
		return nil, nil
	}
	return bps, err
}

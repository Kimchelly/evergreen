package bet

import (
	"github.com/evergreen-ci/evergreen/db"
	"github.com/mongodb/anser/bsonutil"
	adb "github.com/mongodb/anser/db"
	"go.mongodb.org/mongo-driver/bson"
)

const (
	Collection = "bonusly_bets"
)

var (
	IDKey             = bsonutil.MustHaveTag(Bet{}, "ID")
	ParentPoolIDKey   = bsonutil.MustHaveTag(Bet{}, "ParentPoolID")
	UserIDKey         = bsonutil.MustHaveTag(Bet{}, "UserID")
	ExpectedStatusKey = bsonutil.MustHaveTag(Bet{}, "ExpectedStatus")
	AmountKey         = bsonutil.MustHaveTag(Bet{}, "Amount")
	MessageKey        = bsonutil.MustHaveTag(Bet{}, "Message")
)

// ByID returns a query that finds a bet by ID.
func ByID(id string) db.Q {
	return db.Query(bson.M{
		IDKey: id,
	})
}

func ByIDs(ids ...string) db.Q {
	return db.Query(bson.M{
		IDKey: bson.M{"$in": ids},
	})
}

// kim: TODO: maybe remove backreference
// // ByParentPoolID returns a query that finds all bets with the given parent pool
// // ID.
// func ByParentPoolID(id string) db.Q {
//     return db.Query(bson.M{
//         ParentPoolIDKey: id,
//     })
// }

// ByUserID returns a query that finds all bets submitted by the given user.
func ByUserID(id string) db.Q {
	return db.Query(bson.M{
		UserIDKey: id,
	})
}

// FindOne finds a single bet that satisfies the query.
func FindOne(query db.Q) (*Bet, error) {
	var bet Bet
	err := db.FindAllQ(Collection, query, &bet)
	if adb.ResultsNotFound(err) {
		return nil, nil
	}
	return &bet, err
}

// FindAll returns all bets that satisfy the query.
func FindAll(query db.Q) ([]Bet, error) {
	var bets []Bet
	err := db.FindAllQ(Collection, query, &bets)
	if adb.ResultsNotFound(err) {
		return nil, nil
	}
	return bets, nil
}

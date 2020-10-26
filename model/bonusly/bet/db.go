package bonusly

import (
	"github.com/evergreen-ci/evergreen/db"
	"github.com/mongodb/anser/bsonutil"
	adb "github.com/mongodb/anser/db"
)

const (
	Collection = "bonusly_bets"
)

var (
	IDKey             = bsonutil.MustHaveTag(Bet{}, "ID")
	UserIDKey         = bsonutil.MustHaveTag(Bet{}, "UserID")
	ExpectedStatusKey = bsonutil.MustHaveTag(Bet{}, "ExpectedStatus")
	AmountKey         = bsonutil.MustHaveTag(Bet{}, "Amount")
	MessageKey        = bsonutil.MustHaveTag(Bet{}, "Message")
)

// FindOne finds a single bet that satisfies the query.
func FindOne(query db.Q) (*Bet, error) {
	var bet Bet
	err := db.FindAllQ(Collection, query, &bet)
	if adb.ResultsNotFound(err) {
		return nil, nil
	}
	return &bet, nil
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

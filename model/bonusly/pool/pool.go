package pool

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/evergreen-ci/evergreen"
	"github.com/evergreen-ci/evergreen/db"
	"github.com/evergreen-ci/evergreen/model"
	"github.com/evergreen-ci/evergreen/model/bonusly/bet"
	"github.com/evergreen-ci/evergreen/model/task"
	"github.com/evergreen-ci/utility"
	"github.com/kimchelly/go-bonusly"
	"github.com/mongodb/grip"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
)

// BettingPool represents a betting pool for users to wager Bonusly points on
// task outcomes.
type BettingPool struct {
	ID         string   `bson:"_id" json:"id"`
	TaskID     string   `bson:"task_id,omitempty" json:"task_id,omitempty"`
	VersionID  string   `bson:"version_id,omitempty" json:"version_id,omitempty"`
	BetIDs     []string `bson:"bet_ids,omitempty" json:"bet_ids,omitempty"`
	MinimumBet int      `bson:"minimum_bet,omitempty" json:"minimum_bet,omitempty"`
}

var (
	allowedTaskOutcomes = []string{
		evergreen.TaskSucceeded,
		evergreen.TaskFailed,
		evergreen.TaskSystemFailed,
		evergreen.TaskTestTimedOut,
		evergreen.TaskSetupFailed,
	}

	allowedVersionOutcomes = []string{
		evergreen.VersionFailed,
		evergreen.VersionSucceeded,
	}
)

// Validate checks that the betting pool is valid.
func (bp *BettingPool) Validate() error {
	catcher := grip.NewBasicCatcher()

	catcher.NewWhen(bp.MinimumBet < 0, "minimum bet cannot be negative")

	catcher.NewWhen(len(bp.BetIDs) == 0, "no bets found for this pool")
	bets, err := bet.FindAll(bet.ByIDs(bp.BetIDs...))
	if err != nil {
		return errors.Wrapf(err, "finding bets")
	}
	catcher.NewWhen(len(bets) == 0, "no bets found")
	for _, b := range bets {
		catcher.Wrap(bp.ValidateBet(b), "invalid bet")
	}

	catcher.NewWhen(bp.TaskID == "" && bp.VersionID == "", "betting pool is associated with neither a version nor a task")
	catcher.Wrapf(bp.ValidateTask(), "invalid task '%s'", bp.TaskID)
	catcher.Wrapf(bp.ValidateVersion(), "invalid version '%s'", bp.VersionID)

	return errors.Wrapf(catcher.Resolve(), "betting pool '%s'", bp.ID)
}

// ValidateBet checks that a bet is valid to add to this betting pool.
func (bp *BettingPool) ValidateBet(b bet.Bet) error {
	catcher := grip.NewBasicCatcher()

	catcher.Add(b.Validate())
	catcher.ErrorfWhen(b.Amount < bp.MinimumBet, "bet amount is too low, the minimum bet for the pool is %d", bp.MinimumBet)
	catcher.ErrorfWhen(bp.TaskID != "" && !utility.StringSliceContains(allowedTaskOutcomes, b.ExpectedStatus),
		"status '%s' is not a valid task outcome for a bet -  allowed outcome are %s", b.ExpectedStatus, strings.Join(allowedTaskOutcomes, ", "))
	catcher.ErrorfWhen(bp.VersionID != "" && !utility.StringSliceContains(allowedVersionOutcomes, b.ExpectedStatus),
		"status '%s' is not a valid version outcome for a bet - allowed outcomes are %s", b.ExpectedStatus, strings.Join(allowedVersionOutcomes, ", "))

	return errors.Wrapf(catcher.Resolve(), "invalid bet '%s' placed by user '%s'", b.ID, b.UserID)
}

// ValidateVeresion validates that the betting pool references a valid task.
func (bp *BettingPool) ValidateTask() error {
	if bp.TaskID == "" {
		return nil
	}

	t, err := task.FindOne(task.ById(bp.TaskID))
	if err != nil {
		return errors.Wrap(err, "finding task")
	}
	if t == nil {
		return errors.New("could not find task")
	}

	return nil
}

// ValidateVeresion validates that the betting pool references a valid version.
func (bp *BettingPool) ValidateVersion() error {
	if bp.VersionID == "" {
		return nil
	}

	v, err := model.VersionFindOne(model.VersionById(bp.VersionID))
	if err != nil {
		return errors.Wrap(err, "finding version")
	}
	if v == nil {
		return errors.New("could not find version")
	}

	return nil
}

// Insert inserts a new betting pool into the collection.
func (bp *BettingPool) Insert() error {
	return db.Insert(Collection, bp)
}

// AddBet adds a new bet to the betting pool.
func (bp *BettingPool) AddBet(b *bet.Bet) error {
	if b == nil {
		return errors.Errorf("cannot add nil bet")
	}
	if err := bp.ValidateBet(*b); err != nil {
		return errors.Wrap(err, "invalid bet")
	}
	if err := b.Insert(); err != nil {
		return errors.WithStack(err)
	}
	if err := UpdateOne(db.Query(bson.M{
		IDKey: bp.ID,
	}), db.Query(bson.M{
		"$addToSet": bson.M{BetIDsKey: b.ID},
	})); err != nil {
		return errors.Wrap(err, "adding new bet to pool")
	}

	bp.BetIDs = append(bp.BetIDs, b.ID)

	return nil
}

// DecideOutcome determines who is the winner and loser in the betting pool.
func (bp *BettingPool) DecideOutcome(status string) (*Outcome, error) {
	// If we have less than two bets in the pool, there will be no winner/loser.
	if len(bp.BetIDs) < 2 {
		return &Outcome{}, nil
	}
	bets, err := bet.FindAll(bet.ByIDs(bp.BetIDs...))
	if err != nil {
		return nil, errors.Wrapf(err, "finding bets")
	}
	var outcome Outcome
	for _, b := range bets {
		if b.ExpectedStatus == status {
			outcome.Winners = append(outcome.Winners, b)
		} else {
			outcome.Losers = append(outcome.Losers, b)
		}
	}

	return &outcome, nil
}

// Outcome repesents the outcome of a betting pool.
type Outcome struct {
	Winners []bet.Bet
	Losers  []bet.Bet
}

// Validate checks that all losers can distribute their bets to the winners.
func (o *Outcome) Validate() error {
	catcher := grip.NewBasicCatcher()
	for _, loser := range o.Losers {
		catcher.Add(loser.ValidateUser())
	}
	return catcher.Resolve()
}

// Distribute validates that all losers can distribute their earnings and then
// distributes the earnings.
func (o *Outcome) Distribute() error {
	if err := o.Validate(); err != nil {
		return errors.Wrap(err, "betting pool outcome is invalid")
	}

	catcher := grip.NewBasicCatcher()
	for _, loser := range o.Losers {
		catcher.Wrapf(o.distributeBet(loser), "distributing Bonusly points from user '%s'", loser.UserID)
	}
	return nil
}

func (o *Outcome) distributeBet(loser bet.Bet) error {
	u, err := loser.FindUser()
	if err != nil {
		return errors.WithStack(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	c, err := bonusly.NewClient(bonusly.ClientOptions{
		AccessToken: u.Bonusly.AccessToken,
	})
	if err != nil {
		return errors.Wrapf(err, "creating Bonusly client for user '%s'", loser.UserID)
	}
	defer c.Close(ctx)

	catcher := grip.NewBasicCatcher()
	for _, winner := range o.Winners {
		amountPerWinner := loser.Amount / len(o.Winners)
		if _, err := c.CreateBonus(ctx, bonusly.CreateBonusRequest{
			Reason: fmt.Sprintf("+%d @%s %s", amountPerWinner, winner.UserID, loser.Message),
		}); err != nil {
			catcher.Wrapf(err, "distributing '%s' Bonusly points from loser '%s' to winner '%s'", amountPerWinner, loser.UserID, winner.UserID)
		}
	}

	return catcher.Resolve()
}

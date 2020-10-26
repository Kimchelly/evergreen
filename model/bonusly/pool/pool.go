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
	allowedTaskStatuses = []string{
		evergreen.TaskInactive,
		evergreen.TaskStatusBlocked,
		evergreen.TaskStatusPending,
		evergreen.TaskUndispatched,
		evergreen.TaskDispatched,
	}
	allowedVersionStatuses = []string{
		evergreen.VersionFailed,
		evergreen.VersionSucceeded,
	}
)

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

func (bp *BettingPool) ValidateBet(b bet.Bet) error {
	catcher := grip.NewBasicCatcher()

	catcher.Add(b.Validate())

	catcher.ErrorfWhen(b.Amount < bp.MinimumBet, "bet amount is too low, the minimum bet for the pool is %d", bp.MinimumBet)
	catcher.ErrorfWhen(bp.TaskID != "" && !utility.StringSliceContains(allowedTaskStatuses, b.ExpectedStatus),
		"status '%s' is not a valid task status for making a bet -  allowed status are %s", b.ExpectedStatus, strings.Join(allowedTaskStatuses, ", "))
	catcher.ErrorfWhen(bp.VersionID != "" && !utility.StringSliceContains(allowedVersionStatuses, b.ExpectedStatus),
		"status '%s' is not a valid version status for making a bet - allowed statuses are %s", b.ExpectedStatus, strings.Join(allowedVersionStatuses, ","))

	return errors.Wrapf(catcher.Resolve(), "invalid bet '%s' placed by user '%s'", b.ID, b.UserID)
}

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

func (bp *BettingPool) Insert() error {
	return db.Insert(Collection, bp)
}

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
	return nil
}

type Outcome struct {
	Winners []bet.Bet
	Losers  []bet.Bet
}

func (bp *BettingPool) DecideOutcome(status string) (*Outcome, error) {
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

func (o *Outcome) DistributePool() error {
	catcher := grip.NewBasicCatcher()
	for _, loser := range o.Losers {
		catcher.Wrapf(o.distributeOnePoorSoulsMoney(loser), "distributing Bonusly points from user '%s'", loser.UserID)
	}
	return nil
}

func (o *Outcome) distributeOnePoorSoulsMoney(loser bet.Bet) error {
	u, err := model.FindUserByID(loser.UserID)
	if err != nil {
		return errors.Wrapf(err, "finding user '%s'", loser.UserID)
	}
	if u == nil {
		return errors.Errorf("could not find user '%s'")
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

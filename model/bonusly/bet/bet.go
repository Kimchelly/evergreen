package bet

import (
	"context"
	"time"

	"github.com/evergreen-ci/evergreen/db"
	"github.com/evergreen-ci/evergreen/model"
	"github.com/evergreen-ci/evergreen/model/user"
	"github.com/kimchelly/go-bonusly"
	"github.com/mongodb/grip"
	"github.com/pkg/errors"
)

// Bet represents a single user's bet within a betting pool.
type Bet struct {
	ID             string `bson:"_id" json:"id"`
	UserID         string `bson:"user_id" json:"user_id"`
	ExpectedStatus string `bson:"expected_status" json:"expected_status"`
	Amount         int    `bson:"amount" json:"amount"`
	Message        string `bson:"message,omitempty" json:"message,omitempty"`

	user *user.DBUser
}

// Validate checks that the bet is valid.
func (b *Bet) Validate() error {
	catcher := grip.NewBasicCatcher()

	catcher.NewWhen(b.ID == "", "missing bet ID")
	catcher.NewWhen(b.UserID == "", "missing user")
	catcher.Wrap(b.ValidateUser(), "validating user placing bet")

	return catcher.Resolve()
}

// ValidateUser validates that the user who is submitting the bet is valid and
// able to submit the bet amount.
func (b *Bet) ValidateUser() error {
	if b.user == nil {
		u, err := b.FindUser()
		if err != nil {
			return errors.WithStack(err)
		}
		b.user = u
	}

	catcher := grip.NewBasicCatcher()
	if b.user.Bonusly.UserName == "" {
		catcher.Errorf("user '%s' cannot submit bets because they have not set their Bonusly user name", b.UserID)
	}
	if b.user.Bonusly.AccessToken == "" {
		catcher.Errorf("user '%s' cannot submit bets because they have not set their Bonusly access token", b.UserID)
	}
	if b.user.Bonusly.UserName != "" && b.user.Bonusly.AccessToken != "" {
		catcher.Wrap(b.ValidateAmount(), "invalid bet amount")
	}

	return catcher.Resolve()
}

// FindUser finds the user for the bet and caches it internally within the Bet.
func (b *Bet) FindUser() (*user.DBUser, error) {
	if b.user != nil {
		return b.user, nil
	}
	u, err := model.FindUserByID(b.UserID)
	if err != nil {
		return nil, errors.Wrapf(err, "finding user '%s'", b.UserID)
	}
	if u == nil {
		return nil, errors.Errorf("could not find user '%s'", b.UserID)
	}
	b.user = u
	return u, nil
}

// ValidateAmount validates that the user has sufficient Bonusly balance to
// submit the bet amount.
func (b *Bet) ValidateAmount() error {
	if b.user == nil {
		u, err := b.FindUser()
		if err != nil {
			return errors.WithStack(err)
		}
		b.user = u
	}
	if b.Amount <= 0 {
		return errors.New("bet amount must be positive")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	c, err := bonusly.NewClient(bonusly.ClientOptions{
		AccessToken: b.user.Bonusly.AccessToken,
	})
	if err != nil {
		return errors.Wrap(err, "creating Bonusly client")
	}
	defer c.Close(ctx)

	info, err := c.MyUserInfo(ctx)
	if err != nil {
		return errors.Wrapf(err, "getting Bonusly info for user '%s'", b.UserID)
	}
	if info.GivingBalance == nil {
		return errors.Errorf("cannot verify remaining Bonusly balance for user '%s'", b.UserID)
	}
	if info.CanGive == nil {
		return errors.Errorf("cannot verify if user '%s' can give Bonusly points", b.UserID)
	}
	if !*info.CanGive {
		return errors.Errorf("user '%s' cannot give Bonusly", b.UserID)
	}
	if balance := *info.GivingBalance; balance < b.Amount {
		return errors.Errorf("insufficient Bonusly balance for bet: current balance is %d", balance)
	}

	return nil
}

// Insert inserts a new bet into the collection.
func (b *Bet) Insert() error {
	return db.Insert(Collection, b)
}

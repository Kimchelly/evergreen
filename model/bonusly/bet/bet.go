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
	ParentPoolID   string `bson:"parent_pool_id" json:"parent_pool_id"`
	UserID         string `bson:"user_id" json:"user_id"`
	ExpectedStatus string `bson:"expected_status" json:"expected_status"`
	Amount         int    `bson:"amount" json:"amount"`
	Message        string `bson:"message,omitempty" json:"message,omitempty"`
}

func (b *Bet) Validate() error {
	catcher := grip.NewBasicCatcher()
	catcher.NewWhen(b.ID == "", "missing bet ID")
	catcher.NewWhen(b.ParentPoolID == "", "missing parent pool ID")
	catcher.NewWhen(b.UserID == "", "missing user")

	u, err := model.FindUserByID(b.UserID)
	if err != nil {
		catcher.Wrapf(err, "finding user '%s'", b.UserID)
	}
	if u == nil {
		catcher.Errorf("could not find user '%s'", b.UserID)
	}

	if u.Bonusly.UserName == "" {
		catcher.Errorf("user '%s' cannot submit bets because they have not set their Bonusly user name", b.UserID)
	} else if u.Bonusly.AccessToken == "" {
		catcher.Errorf("user '%s' cannot submit bets because they have not set their Bonusly access token", b.UserID)
	} else {
		catcher.Wrap(b.ValidateAmount(u), "invalid bet amount")
	}

	return catcher.Resolve()
}

func (b *Bet) ValidateAmount(u *user.DBUser) error {
	if b.Amount <= 0 {
		return errors.New("bet amount must be positive")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	c, err := bonusly.NewClient(bonusly.ClientOptions{
		AccessToken: u.Bonusly.AccessToken,
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
	if balance := *info.GivingBalance; balance < b.Amount {
		return errors.Errorf("insufficient Bonusly balance for bet: current balance is %d", balance)
	}

	return nil
}

func (b *Bet) Insert() error {
	return db.Insert(Collection, b)
}

// kim: TODO: should people be allowed to modify/delete their bets after they've
// placed them? Maybe not.
// func (b *Bet) Upsert() error {
//     _, err := db.Upsert(Collection, ByID(b.ID), bson.M{
//         "$set": bson.M{
//             ParentPoolIDKey:   b.ParentPoolID,
//             UserIDKey:         b.UserID,
//             ExpectedStatusKey: b.ExpectedStatus,
//             AmountKey:         b.Amount,
//             MessageKey:        b.Message,
//         },
//     })
//     if err != nil {
//         return errors.WithStack(err)
//     }
//     return nil
// }

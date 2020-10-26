package pool

// BettingPool represents a betting pool for users to wager Bonusly points on
// task outcomes.
type BettingPool struct {
	ID         string   `bson:"_id" json:"id"`
	TaskID     string   `bson:"task_id,omitempty" json:"task_id,omitempty"`
	VersionID  string   `bson:"version_id,omitempty" json:"version_id,omitempty"`
	BetIDs     []string `bson:"bet_ids,omitempty" json:"bet_ids,omitempty"`
	MinimumBet int      `bson:"minimum_bet,omitempty" json:"minimum_bet,omitempty"`
}

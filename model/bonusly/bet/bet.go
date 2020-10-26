package bonusly

// Bet represents a single user's bet within a betting pool.
type Bet struct {
	ID             string `bson:"_id" json:"id"`
	UserID         string `bson:"user_id" json:"user_id"`
	ExpectedStatus string `bson:"expected_status" json:"expected_status"`
	Amount         int    `bson:"amount" json:"amount"`
	Message        string `bson:"message,omitempty" json:"message,omitempty"`
}

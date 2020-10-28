package event

import (
	"time"

	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
)

const (
	ResourceTypeBonuslyBet = "BONUSLY"

	EventBonuslyBetUserMentioned = "USER_MENTIONED"
)

type BonuslyBetEventData struct {
	User          string `bson:"user,omitempty" json:"user,omitempty"`
	MentionedUser string `bson:"mentioned_user,omitempty" json:"mentioned_user,omitempty"`
}

func LogBonuslyBetEvent(bettingPoolID, eventType string, eventData BonuslyBetEventData) {
	event := EventLogEntry{
		Timestamp:    time.Now(),
		ResourceId:   bettingPoolID,
		EventType:    eventType,
		Data:         eventData,
		ResourceType: ResourceTypeBonuslyBet,
	}

	logger := NewDBEventLogger(AllLogCollection)
	if err := logger.LogEvent(&event); err != nil {
		grip.Error(message.WrapError(err, message.Fields{
			"message":       "error logging event",
			"resource_type": ResourceTypeBonuslyBet,
			"source":        "event-log-fail",
		}))
	}
}

func LogBonuslyBetUserMentioned(bettingPoolID, user, mentionedUser string) {
	LogBonuslyBetEvent(bettingPoolID, EventBonuslyBetUserMentioned, BonuslyBetEventData{
		User:          user,
		MentionedUser: mentionedUser,
	})
}

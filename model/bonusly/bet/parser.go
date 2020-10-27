package bet

import (
	"strconv"
	"strings"

	"github.com/evergreen-ci/evergreen"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// kim: TODO: move parser to comment package.

const (
	betCommand = "/bet"
)

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

// IsBonuslyBet returns whether or not the comment message is a Bonusly bet.
func IsBonuslyBet(message string) bool {
	return strings.HasPrefix(strings.TrimSpace(message), betCommand)
}

// ParseComment parses the Bonusly bet command and checks that it is valid.
func ParseComment(message, userID string) (*Bet, error) {
	var err error
	if message, err = parseBetCommand(message); err != nil {
		return nil, errors.WithStack(err)
	}
	var status string
	if message, status, err = parseExpectedStatus(message); err != nil {
		return nil, errors.WithStack(err)
	}
	// kim: TODO: handle parsing and notifying users on Slack.
	// if message, err = parseOptionalUsers(message); err != nil {
	// }
	var amount int
	if message, amount, err = parseAmount(message); err != nil {
		return nil, errors.WithStack(err)
	}

	b := Bet{
		ID:             primitive.NewObjectID().String(),
		UserID:         userID,
		ExpectedStatus: status,
		Amount:         amount,
		Message:        strings.TrimSpace(message),
	}
	if err := b.Validate(); err != nil {
		return nil, errors.Wrap(err, "validating bet")
	}

	return &b, nil
}

func parseBetCommand(message string) (string, error) {
	message = strings.TrimSpace(message)
	if !strings.HasPrefix(message, betCommand) {
		return message, errors.Errorf("missing Bonusly %s command", betCommand)
	}
	return strings.TrimPrefix(message, betCommand), nil
}

func parseExpectedStatus(message string) (parsed string, status string, err error) {
	message = strings.TrimSpace(message)

	for _, status := range allowedTaskStatuses {
		if strings.HasPrefix(message, status) {
			return strings.TrimPrefix(message, status), status, nil
		}
	}

	for _, status := range allowedVersionStatuses {
		if strings.HasPrefix(message, status) {
			return strings.TrimPrefix(message, status), status, nil
		}
	}

	return message, "", errors.New("bet does not have a valid expected status to bet on")
}

func parseAmount(message string) (parsed string, amount int, err error) {
	message = strings.TrimSpace(message)

	fields := strings.Fields(message)
	if len(fields) == 0 {
		return message, -1, errors.Errorf("could not parse bet amount")
	}
	if strings.HasPrefix(fields[0], "+") {
		message = strings.TrimPrefix(message, "+")
	}
	if amount, err = strconv.Atoi(fields[0]); err != nil {
		return message, -1, errors.Wrap(err, "parsing bet as an integer")
	}

	return strings.TrimPrefix(message, fields[0]), amount, nil
}

package bonusly

import (
	"strconv"
	"strings"

	"github.com/evergreen-ci/evergreen"
	"github.com/evergreen-ci/evergreen/model/bonusly/bet"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

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
func IsBonuslyBet(comment string) bool {
	return strings.HasPrefix(strings.TrimSpace(comment), betCommand)
}

// ParsedBettingPool represents the options parsed from a betting pool comment.
type ParsedBettingPool struct {
	ParsedBet
	MinimumBet int
}

// ParseBettingPoolComment parses a Bonusly bet comment to initialize a betting
// pool.
// Valid betting comments take the form: /bet <expected_outcome> [+]amount [+]minBet [@user1, @user2...] [comment]
func ParseBettingPoolComment(comment, userID string) (*ParsedBettingPool, error) {
	var err error
	if comment, err = parseBetCommand(comment); err != nil {
		return nil, errors.WithStack(err)
	}

	var status string
	if comment, status, err = parseExpectedStatus(comment); err != nil {
		return nil, errors.WithStack(err)
	}

	var amount int
	if comment, amount, err = parseAmount(comment); err != nil {
		return nil, errors.WithStack(err)
	}

	var minBet int
	comment, minBet, _ = parseAmount(comment)
	if minBet == -1 {
		minBet = 0
	}

	var userMentions []string
	comment, userMentions = parseUserMentions(comment)

	return &ParsedBettingPool{
		ParsedBet: ParsedBet{
			Bet: bet.Bet{
				ID:             primitive.NewObjectID().Hex(),
				UserID:         userID,
				ExpectedStatus: status,
				Amount:         amount,
				Message:        strings.TrimSpace(comment),
			},

			UserMentions: userMentions,
		},
		MinimumBet: minBet,
	}, nil
}

// ParsedBettingPool represents the options parsed from a Bonusly bet comment.
type ParsedBet struct {
	Bet          bet.Bet
	UserMentions []string
}

// ParseBetComment parses a Bonusly bet comment.
func ParseBetComment(comment, userID string) (*ParsedBet, error) {
	var err error
	if comment, err = parseBetCommand(comment); err != nil {
		return nil, errors.WithStack(err)
	}
	var status string
	if comment, status, err = parseExpectedStatus(comment); err != nil {
		return nil, errors.WithStack(err)
	}
	var amount int
	if comment, amount, err = parseAmount(comment); err != nil {
		return nil, errors.WithStack(err)
	}

	var userMentions []string
	comment, userMentions = parseUserMentions(comment)

	return &ParsedBet{
		Bet: bet.Bet{
			ID:             primitive.NewObjectID().Hex(),
			UserID:         userID,
			ExpectedStatus: status,
			Amount:         amount,
			Message:        strings.TrimSpace(comment),
		},
		UserMentions: userMentions,
	}, nil
}

func parseBetCommand(comment string) (parsed string, err error) {
	parsed = strings.TrimSpace(comment)
	if !strings.HasPrefix(parsed, betCommand) {
		return parsed, errors.Errorf("missing Bonusly %s command", betCommand)
	}
	return strings.TrimPrefix(parsed, betCommand), nil
}

func parseExpectedStatus(comment string) (parsed string, status string, err error) {
	parsed = strings.TrimSpace(comment)

	for _, status := range allowedTaskStatuses {
		if strings.HasPrefix(parsed, status) {
			return strings.TrimPrefix(parsed, status), status, nil
		}
	}

	for _, status := range allowedVersionStatuses {
		if strings.HasPrefix(parsed, status) {
			return strings.TrimPrefix(parsed, status), status, nil
		}
	}

	return comment, "", errors.New("bet does not have a valid expected status to bet on")
}

func parseAmount(comment string) (parsed string, amount int, err error) {
	parsed = strings.TrimSpace(comment)

	words := strings.Fields(parsed)
	if len(words) == 0 {
		return comment, -1, errors.Errorf("could not parse bet amount")
	}
	if amount, err = strconv.Atoi(words[0]); err != nil {
		return comment, -1, errors.Wrap(err, "parsing bet as an integer")
	}

	return strings.TrimPrefix(parsed, words[0]), amount, nil
}

func parseUserMentions(comment string) (parsed string, mentions []string) {
	parsed = comment

	parsed = strings.TrimSpace(parsed)
	words := strings.Fields(parsed)

	for _, word := range words {
		if !strings.HasPrefix(word, "@") {
			break
		}
		user := strings.TrimPrefix(word, "@")
		mentions = append(mentions, user)
		parsed = strings.TrimPrefix(strings.TrimSpace(parsed), word)
	}

	if len(mentions) == 0 {
		return comment, nil
	}

	return parsed, mentions
}

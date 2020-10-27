package parser

import (
	"testing"

	"github.com/evergreen-ci/evergreen"
	"github.com/evergreen-ci/evergreen/model/bonusly/bet"
	"github.com/evergreen-ci/evergreen/testutil"
	"github.com/evergreen-ci/utility"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	testutil.Setup()
}

func TestParseBettingPoolComment(t *testing.T) {
	for testName, testCase := range map[string]struct {
		comment      string
		userID       string
		expectedOpts BettingPoolOptions
		expectedBet  bet.Bet
		expectError  bool
	}{
		"SucceedsWithBasicRequiredArguments": {
			comment:      "/bet success +10 lol what",
			userID:       "me",
			expectedOpts: BettingPoolOptions{},
			expectedBet: bet.Bet{
				ExpectedStatus: evergreen.TaskSucceeded,
				Amount:         10,
				Message:        "lol what",
			},
		},
		"SucceedsWithoutMessage": {
			comment:      "/bet success +10",
			userID:       "me",
			expectedOpts: BettingPoolOptions{},
			expectedBet: bet.Bet{
				ExpectedStatus: evergreen.TaskSucceeded,
				Amount:         10,
			},
		},
		"FailsWithoutExpectedOutcome": {
			comment:     "/bet +10 bad bet",
			userID:      "me",
			expectError: true,
		},
		"FailsWithoutBetAmount": {
			comment:     "/bet success bad bet",
			userID:      "me",
			expectError: true,
		},
		"FailsWithCommentWithoutBetAmount": {
			comment:     "/bet success bad bet",
			userID:      "me",
			expectError: true,
		},
		"SucceedsWithMinimumBet": {
			comment: "/bet failed +30 10 lol",
			userID:  "me",
			expectedOpts: BettingPoolOptions{
				MinimumBet: 10,
			},
			expectedBet: bet.Bet{
				ExpectedStatus: evergreen.TaskFailed,
				Amount:         30,
				Message:        "lol",
			},
		},
		"SucceedsWithUserMention": {
			comment: "/bet failed +15 @somebody bananas",
			userID:  "me",
			expectedOpts: BettingPoolOptions{
				UserMentions: []string{"somebody"},
			},
			expectedBet: bet.Bet{
				ExpectedStatus: evergreen.TaskFailed,
				Amount:         15,
				Message:        "bananas",
			},
		},
		"SucceedsWithMultipleUserMentions": {
			comment: "/bet success +15 @somebody @somebody-else this skunkworks project is 2 ez",
			userID:  "me",
			expectedOpts: BettingPoolOptions{
				UserMentions: []string{"somebody", "somebody-else"},
			},
			expectedBet: bet.Bet{
				ExpectedStatus: evergreen.TaskSucceeded,
				Amount:         15,
				Message:        "this skunkworks project is 2 ez",
			},
		},
		"SucceedsWithMinimumBetAndUserMention": {
			comment: "/bet failed +15 10 @somebody torchlight 3 < torchlight 2",
			userID:  "me",
			expectedOpts: BettingPoolOptions{
				MinimumBet:   10,
				UserMentions: []string{"somebody"},
			},
			expectedBet: bet.Bet{
				ExpectedStatus: evergreen.TaskFailed,
				Amount:         15,
				Message:        "torchlight 3 < torchlight 2",
			},
		},
	} {
		t.Run(testName, func(t *testing.T) {
			bpOpts, b, err := ParseBettingPoolComment(testCase.comment, testCase.userID)
			if testCase.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, bpOpts)
				require.NotNil(t, b)

				assert.Equal(t, testCase.expectedOpts.MinimumBet, bpOpts.MinimumBet)
				assert.Equal(t, testCase.expectedOpts.UserMentions, bpOpts.UserMentions)
				missingExpected, missingActual := utility.StringSliceSymmetricDifference(testCase.expectedOpts.UserMentions, bpOpts.UserMentions)
				assert.Empty(t, missingExpected)
				assert.Empty(t, missingActual)

				assert.NotZero(t, b.ID)
				assert.Equal(t, testCase.userID, b.UserID)
				assert.Equal(t, testCase.expectedBet.ExpectedStatus, b.ExpectedStatus)
				assert.Equal(t, testCase.expectedBet.Amount, b.Amount)
				assert.Equal(t, testCase.expectedBet.Message, b.Message)
			}
		})
	}
}

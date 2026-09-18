package scoring_test

import (
	"testing"

	"github.com/cabon-tech/not-my-sim/internal/scoring"
)

func TestEvaluate_CEPOnly(t *testing.T) {
	tests := []struct {
		state          string
		expectedScore  int
		expectedAction scoring.Action
	}{
		{"NORMAL", 0, scoring.ActionLogOnly},
		{"SWAP_DETECTED", 40, scoring.ActionSMSAlert},
		{"CREDENTIAL_RESET_POST_SWAP", 70, scoring.ActionSMSAndStepUp},
		{"TRANSFER_POST_RESET", 90, scoring.ActionSMSAndHold},
	}

	for _, tc := range tests {
		// Pass nil DB to bypass the Graph/Hub scoring component and test CEP scoring in isolation.
		res := scoring.Evaluate(nil, "+254700000001", tc.state, "", "")
		if res.Score != tc.expectedScore {
			t.Errorf("state %s: expected score %d, got %d", tc.state, tc.expectedScore, res.Score)
		}
		if res.Action != tc.expectedAction {
			t.Errorf("state %s: expected action %s, got %s", tc.state, tc.expectedAction, res.Action)
		}
	}
}

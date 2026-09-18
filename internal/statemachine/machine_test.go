package statemachine_test

import (
	"testing"

	"github.com/cabon-tech/not-my-sim/internal/statemachine"
)

func TestRiskLevels(t *testing.T) {
	tests := []struct {
		state    statemachine.State
		expected int
	}{
		{statemachine.StateNormal, 0},
		{statemachine.StateSwapDetected, 40},
		{statemachine.StateCredentialResetPostSwap, 70},
		{statemachine.StateTransferPostReset, 90},
	}

	for _, tc := range tests {
		if got := tc.state.RiskLevel(); got != tc.expected {
			t.Errorf("expected %d for %s, got %d", tc.expected, tc.state, got)
		}
	}
}

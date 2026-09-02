package engine

import "testing"

func TestDetectCycleReturnsExactPeriodAndPathBoundary(t *testing.T) {
	visits := []StateVisit{
		{Sequence: 10, StateIdentity: "a"},
		{Sequence: 11, StateIdentity: "b"},
		{Sequence: 12, StateIdentity: "a"},
		{Sequence: 13, StateIdentity: "b"},
	}
	start, period, ok := detectCycle(visits)
	if !ok || start != 0 || period != 2 {
		t.Fatalf("detectCycle() = (%d, %d, %t), want (0, 2, true)", start, period, ok)
	}
}

func TestUnknownRecordAlwaysHasOperationalFields(t *testing.T) {
	record := unknownFromDecision(CellDecision{})
	if record.Stage == "" || record.Step == "" || record.Reason == "" || record.UnknownClass == "" || record.NextOperation == "" || record.BlockedBy == "" {
		t.Fatalf("unknown record is incomplete: %+v", record)
	}
}

func TestDominantDecisionUsesAuthorityPrecedence(t *testing.T) {
	got := dominantDecision([]Decision{DecisionClosed, DecisionUnknown, DecisionRefuted})
	if got != DecisionRefuted {
		t.Fatalf("dominantDecision() = %s, want REFUTED", got)
	}
}

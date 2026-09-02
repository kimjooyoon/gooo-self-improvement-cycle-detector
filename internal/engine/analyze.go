package engine

import (
	"encoding/json"
	"fmt"
	"sort"
)

func Analyze(authorityJSON []byte, name string, trace Trace, claims []Claim) (Report, error) {
	spec, authority, err := ParseSpec(authorityJSON)
	if err != nil {
		return Report{}, err
	}
	results := make([]CellResult, len(spec.Cells))
	frontiers := make([]*CausalFrontier, len(spec.Cells))
	cellIndexes := make(map[string]int, len(spec.Cells))
	for index, cell := range spec.Cells {
		cellIndexes[cell.ID] = index
		results[index] = CellResult{
			CellID:      cell.ID,
			ProofChoice: cell.ProofChoice,
			Indicator:   cell.Indicator,
			Activity:    cell.Activity,
			Decision:    DecisionClosed,
		}
	}
	report := Report{
		Schema:    "gooo.trace.detector.report/v1",
		CaseName: name,
		Authority: authority,
		Detections: []Detection{},
		CausalFrontier: []CausalFrontier{},
		OperationalRefuted: []OperationalEvent{},
	}
	if len(trace.Events) == 0 {
		for index := range results {
			setUnknown(results, frontiers, index, unknown("INGEST", "read events", "trace has no events", "incomplete_trace", "SUPPLY_APPEND_ONLY_TRACE", "trace.events"), frontier(0, "", "missing_state", ""))
		}
		return finalizeReport(report, results, frontiers), nil
	}

	stateVisits := make([]StateVisit, 0, len(trace.Events))
	sequenceValid := true
	allEvidenceValid := true
	allStatesPresent := true
	allDecisionSetsValid := true
	missingEvidence := false
	staleEvidence := false
	regressionObserved := false
	lastSequence := 0
	for eventIndex, event := range trace.Events {
		if eventIndex > 0 && event.Sequence <= lastSequence {
			sequenceValid = false
			for index := range results {
				setUnknown(results, frontiers, index, unknown("INGEST", "order events", "event sequence is not strictly increasing", "ambiguous_ordering", "RECORD_MONOTONIC_SEQUENCE", "trace.events.sequence"), frontier(event.Sequence, event.StateIdentity, "ambiguous_ordering", ""))
			}
		}
		lastSequence = event.Sequence
		stateVisits = append(stateVisits, StateVisit{Sequence: event.Sequence, StateIdentity: event.StateIdentity})
		if event.StateIdentity == "" {
			allStatesPresent = false
			for index := range results {
				setUnknown(results, frontiers, index, unknown("INGEST", "bind state", "semantic state identity is missing", "missing_state", "SUPPLY_COMPLETE_SEMANTIC_STATE", "event.state_identity"), frontier(event.Sequence, "", "missing_state", ""))
			}
		}
		if event.EvaluatorIdentity == "" || event.ToolchainIdentity == "" || event.ChangeReceipt.Identity == "" {
			allEvidenceValid = false
		}
		seenDecisions := make(map[string]bool, len(event.Decisions))
		for _, decision := range event.Decisions {
			index, ok := cellIndexes[decision.CellID]
			if !ok || seenDecisions[decision.CellID] {
				allDecisionSetsValid = false
				continue
			}
			seenDecisions[decision.CellID] = true
			if decision.Decision != DecisionClosed && decision.Decision != DecisionUnknown && decision.Decision != DecisionRefuted {
				allDecisionSetsValid = false
				setUnknown(results, frontiers, index, unknown("CLASSIFY", "validate decision", "cell decision is not a recognized decision", "incomplete_trace", "RECORD_VALID_CELL_DECISION", "event.decisions"), frontier(event.Sequence, event.StateIdentity, "incomplete_trace", decision.EvidenceIdentity))
			}
			if decision.StateIdentity == "" || decision.EvidenceIdentity == "" || decision.EvidenceStateIdentity == "" || decision.EvaluatorIdentity == "" || decision.ToolchainIdentity == "" || decision.ChangeReceiptIdentity == "" {
				allEvidenceValid = false
				class := "missing_evidence"
				operation := "COLLECT_REQUIRED_EVIDENCE_IDENTITIES"
				blockedBy := "cell_decision.evidence_identity"
				if decision.StateIdentity == "" || event.StateIdentity == "" {
					class = "missing_state"
					operation = "SUPPLY_COMPLETE_SEMANTIC_STATE"
					blockedBy = "event.state_identity"
				} else {
					missingEvidence = true
				}
				setUnknown(results, frontiers, index, unknown("INGEST", "validate evidence", "one or more required evidence identities are missing", class, operation, blockedBy), frontier(event.Sequence, event.StateIdentity, class, decision.EvidenceIdentity))
			}
			if decision.StateIdentity != event.StateIdentity || (decision.EvidenceStateIdentity != "" && decision.EvidenceStateIdentity != event.StateIdentity) {
				allEvidenceValid = false
				staleEvidence = true
				setUnknown(results, frontiers, index, unknown("PAIR", "pair state evidence", "evidence is attached to a different semantic state identity", "stale_evidence", "REPLAY_WITH_CURRENT_STATE_EVIDENCE", "cell_decision.evidence_state_identity"), frontier(event.Sequence, event.StateIdentity, "stale_evidence", decision.EvidenceIdentity))
			}
			if decision.EvaluatorIdentity != event.EvaluatorIdentity || decision.ToolchainIdentity != event.ToolchainIdentity || decision.ChangeReceiptIdentity != event.ChangeReceipt.Identity {
				allEvidenceValid = false
				staleEvidence = true
				setUnknown(results, frontiers, index, unknown("PAIR", "pair evaluator receipt", "cell evidence identity does not match the enclosing event identities", "stale_evidence", "REPLAY_WITH_CURRENT_STATE_EVIDENCE", "event.evaluator_identity/event.change_receipt.identity"), frontier(event.Sequence, event.StateIdentity, "stale_evidence", decision.EvidenceIdentity))
			}
			if decision.Decision == DecisionRefuted || decision.SemanticEffect == "REGRESSION" {
				kind := "explicit_refutation"
				reason := "cell decision is explicitly refuted"
				if decision.SemanticEffect == "REGRESSION" {
					kind = "per_cell_regression"
					reason = "paired semantic observation records a regression"
					regressionObserved = true
				}
				setRefuted(results, frontiers, index, Refutation{Kind: kind, Reason: reason}, frontier(event.Sequence, event.StateIdentity, kind, decision.EvidenceIdentity))
				report.OperationalRefuted = append(report.OperationalRefuted, OperationalEvent{
					Event: "OPERATIONAL_REFUTED", CellID: decision.CellID, Sequence: event.Sequence,
					StateIdentity: event.StateIdentity, Reason: reason, NextOperation: "ISOLATE_CAUSAL_CHANGE",
				})
			}
			if decision.Decision == DecisionUnknown {
				setUnknown(results, frontiers, index, unknownFromDecision(decision), frontier(event.Sequence, event.StateIdentity, "incomplete_trace", decision.EvidenceIdentity))
			}
		}
		if len(seenDecisions) != len(spec.Cells) || len(event.Decisions) != len(spec.Cells) {
			allDecisionSetsValid = false
			for index, cell := range spec.Cells {
				if !seenDecisions[cell.ID] {
					setUnknown(results, frontiers, index, unknown("INGEST", "bind cell decisions", "trace event does not contain exactly one decision for every conformance cell", "incomplete_trace", "RECORD_COMPLETE_CELL_VECTOR", "event.decisions"), frontier(event.Sequence, event.StateIdentity, "incomplete_trace", ""))
				}
			}
		}
	}
	if !sequenceValid {
		report.Detections = append(report.Detections, Detection{Kind: "AMBIGUOUS_ORDERING", MinimalCausalFrontier: collectFrontiers(results)})
	}
	if !allStatesPresent {
		report.Detections = append(report.Detections, Detection{Kind: "MISSING_STATE", MinimalCausalFrontier: collectFrontiers(results)})
	}
	if !allDecisionSetsValid {
		report.Detections = append(report.Detections, Detection{Kind: "INCOMPLETE_TRACE", MinimalCausalFrontier: collectFrontiers(results)})
	}
	if missingEvidence {
		report.Detections = append(report.Detections, Detection{Kind: "MISSING_EVIDENCE", MinimalCausalFrontier: collectFrontiers(results)})
	}
	if staleEvidence {
		report.Detections = append(report.Detections, Detection{Kind: "STALE_EVIDENCE", MinimalCausalFrontier: collectFrontiers(results)})
	}
	if regressionObserved {
		report.Detections = append(report.Detections, Detection{Kind: "PER_CELL_REGRESSION", MinimalCausalFrontier: collectFrontiers(results)})
	}

	evaluatorIdentities := uniqueEventValues(trace.Events, func(event TraceEvent) string { return event.EvaluatorIdentity })
	if len(evaluatorIdentities) > 1 {
		for index := range results {
			setUnknown(results, frontiers, index, unknown("PAIR", "hold evaluator constant", "evaluator identity changed within the append-only trace", "evaluator_drift", "PAIR_WITH_ONE_EVALUATOR_IDENTITY", "trace.events.evaluator_identity"), frontier(trace.Events[1].Sequence, trace.Events[1].StateIdentity, "evaluator_drift", ""))
		}
		report.Detections = append(report.Detections, Detection{Kind: "EVALUATOR_DRIFT", MinimalCausalFrontier: collectFrontiers(results)})
	}
	toolchainIdentities := uniqueEventValues(trace.Events, func(event TraceEvent) string { return event.ToolchainIdentity })
	if len(toolchainIdentities) > 1 {
		for index := range results {
			setUnknown(results, frontiers, index, unknown("PAIR", "hold toolchain constant", "toolchain identity changed within the append-only trace", "toolchain_drift", "PAIR_WITH_ONE_TOOLCHAIN_IDENTITY", "trace.events.toolchain_identity"), frontier(trace.Events[1].Sequence, trace.Events[1].StateIdentity, "toolchain_drift", ""))
		}
		report.Detections = append(report.Detections, Detection{Kind: "TOOLCHAIN_DRIFT", MinimalCausalFrontier: collectFrontiers(results)})
	}

	counterexamples := counterexampleOccurrences(trace)
	keys := make([]string, 0, len(counterexamples))
	for key := range counterexamples {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	counterexampleObserved := false
	for _, key := range keys {
		occurrences := counterexamples[key]
		if len(occurrences) < 2 {
			continue
		}
		cellID := keyCellID(key)
		index, ok := cellIndexes[cellID]
		if !ok {
			continue
		}
		counterexampleObserved = true
		ids := make([]string, 0, len(occurrences))
		for _, occurrence := range occurrences {
			ids = append(ids, occurrence.Identity)
		}
		setRefuted(results, frontiers, index, Refutation{Kind: "counterexample_recurrence", Reason: "the same counterexample identity recurs", CounterexampleIDs: ids}, frontier(occurrences[1].Sequence, occurrences[1].StateIdentity, "counterexample_recurrence", occurrences[1].EvidenceIdentity))
		report.OperationalRefuted = append(report.OperationalRefuted, OperationalEvent{
			Event: "OPERATIONAL_REFUTED", CellID: cellID, Sequence: occurrences[1].Sequence,
			StateIdentity: occurrences[1].StateIdentity, Reason: "counterexample identity recurred", NextOperation: "ISOLATE_COUNTEREXAMPLE_CAUSE",
		})
	}
	if counterexampleObserved {
		report.Detections = append(report.Detections, Detection{Kind: "COUNTEREXAMPLE_RECURRENCE", MinimalCausalFrontier: collectFrontiers(results)})
	}

	hasStateChange := hasStateChange(trace.Events)
	claimStatus := validateClaims(spec, trace, claims)
	for index, cell := range spec.Cells {
		if hasStateChange {
			status, ok := claimStatus[cell.ID]
			if !ok {
				setUnknown(results, frontiers, index, unknown("PAIR", "pair improvement evidence", "no exact paired evidence claim exists for this cell", "insufficient_observation", "PAIR_EXACT_STATE_EVIDENCE", "claims"), frontier(trace.Events[len(trace.Events)-1].Sequence, trace.Events[len(trace.Events)-1].StateIdentity, "insufficient_observation", ""))
			} else if !status {
				setUnknown(results, frontiers, index, unknown("PAIR", "pair improvement evidence", "improvement claim does not match both exact evidence identities", "insufficient_observation", "PAIR_EXACT_STATE_EVIDENCE", "claims.from_evidence_identity/to_evidence_identity"), frontier(trace.Events[len(trace.Events)-1].Sequence, trace.Events[len(trace.Events)-1].StateIdentity, "insufficient_observation", ""))
			}
		}
	}

	cycleStart, period, hasCycle := detectCycle(stateVisits)
	fixedPointAccepted := fixedPointMatches(trace, spec, allEvidenceValid && allStatesPresent && allDecisionSetsValid && sequenceValid)
	if trace.FixedPoint != nil && trace.FixedPoint.Authorized && !fixedPointAccepted {
		for index := range results {
			setUnknown(results, frontiers, index, unknown("CLASSIFY", "verify fixed point", "authorized fixed point lacks matching required evidence identities", "fixed_point_evidence_mismatch", "REAUTHORIZE_FIXED_POINT_WITH_MATCHING_EVIDENCE", "trace.fixed_point.evidence_identities"), frontier(trace.Events[len(trace.Events)-1].Sequence, trace.Events[len(trace.Events)-1].StateIdentity, "fixed_point_evidence_mismatch", ""))
		}
	}
	if fixedPointAccepted {
		report.Detections = append(report.Detections, Detection{
			Kind: "EXPLICIT_FIXED_POINT", Period: 1, CycleStartSequence: trace.Events[0].Sequence,
			RepeatedStatePath: append([]StateVisit(nil), stateVisits...), MinimalCausalFrontier: collectFrontiers(results),
		})
	} else if hasCycle {
		path := append([]StateVisit(nil), stateVisits[cycleStart:]...)
		kind := "PERIOD_N_CYCLE"
		unknownClass := "cycle"
		nextOperation := "BREAK_CYCLE_WITH_CAUSAL_CHANGE"
		if period == 1 {
			kind = "REPEATED_NOOP"
			unknownClass = "repeated_noop"
			nextOperation = "AUTHORIZE_FIXED_POINT_OR_CHANGE_STATE"
		} else if period == 2 {
			kind = "PERIOD_2_OSCILLATION"
			unknownClass = "oscillation"
			nextOperation = "BREAK_OSCILLATION_WITH_CAUSAL_CHANGE"
		}
		firstRepeatIndex := cycleStart + period
		if firstRepeatIndex >= len(trace.Events) {
			firstRepeatIndex = len(trace.Events) - 1
		}
		for index := range results {
			setUnknown(results, frontiers, index, unknown("CLASSIFY", "classify repeated state path", fmt.Sprintf("state identities repeat with exact period %d", period), unknownClass, nextOperation, "trace.events.state_identity"), frontier(trace.Events[firstRepeatIndex].Sequence, trace.Events[firstRepeatIndex].StateIdentity, kind, ""))
		}
		detection := Detection{Kind: kind, Period: period, CycleStartSequence: trace.Events[cycleStart].Sequence, RepeatedStatePath: path, MinimalCausalFrontier: collectFrontiers(results)}
		report.Detections = append(report.Detections, detection)
	}

	if fixedPointAccepted {
		for index := range results {
			if results[index].Decision == DecisionUnknown && results[index].Refutation == nil {
				continue
			}
			if results[index].Decision == DecisionClosed {
				results[index].Unknown = nil
			}
		}
	}
	return finalizeReport(report, results, frontiers), nil
}

func finalizeReport(report Report, results []CellResult, frontiers []*CausalFrontier) Report {
	report.Cells = results
	report.DecisionVector = make([]Decision, len(results))
	for index := range results {
		report.DecisionVector[index] = results[index].Decision
		if frontiers[index] != nil {
			results[index].CausalFrontier = frontiers[index]
		}
	}
	report.Cells = results
	report.CaseDecision = dominantDecision(report.DecisionVector)
	report.CausalFrontier = collectFrontiers(results)
	return report
}

func unknown(stage, step, reason, class, nextOperation, blockedBy string) UnknownRecord {
	return UnknownRecord{Stage: stage, Step: step, Reason: reason, UnknownClass: class, NextOperation: nextOperation, BlockedBy: blockedBy}
}

func unknownFromDecision(decision CellDecision) UnknownRecord {
	record := unknown(decision.Stage, decision.Step, decision.Reason, decision.UnknownClass, decision.NextOperation, decision.BlockedBy)
	if record.Stage == "" {
		record.Stage = "CLASSIFY"
	}
	if record.Step == "" {
		record.Step = "classify unknown decision"
	}
	if record.Reason == "" {
		record.Reason = "cell decision is unknown"
	}
	if record.UnknownClass == "" {
		record.UnknownClass = "incomplete_trace"
	}
	if record.NextOperation == "" {
		record.NextOperation = "RECORD_UNKNOWN_CAUSE"
	}
	if record.BlockedBy == "" {
		record.BlockedBy = "cell_decision"
	}
	return record
}

func frontier(sequence int, stateIdentity, cause, evidence string) CausalFrontier {
	return CausalFrontier{Sequence: sequence, StateIdentity: stateIdentity, Cause: cause, EvidenceIdentity: evidence}
}

func setUnknown(results []CellResult, frontiers []*CausalFrontier, index int, record UnknownRecord, candidate CausalFrontier) {
	if results[index].Decision == DecisionRefuted {
		return
	}
	if results[index].Unknown != nil {
		return
	}
	results[index].Decision = DecisionUnknown
	record = unknownFromDecision(CellDecision{Stage: record.Stage, Step: record.Step, Reason: record.Reason, UnknownClass: record.UnknownClass, NextOperation: record.NextOperation, BlockedBy: record.BlockedBy})
	results[index].Unknown = &record
	frontiers[index] = &candidate
	results[index].CausalFrontier = &candidate
}

func setRefuted(results []CellResult, frontiers []*CausalFrontier, index int, refutation Refutation, candidate CausalFrontier) {
	results[index].Decision = DecisionRefuted
	results[index].Unknown = nil
	results[index].Refutation = &refutation
	if frontiers[index] == nil || candidate.Sequence < frontiers[index].Sequence {
		frontiers[index] = &candidate
		results[index].CausalFrontier = &candidate
	}
}

func collectFrontiers(results []CellResult) []CausalFrontier {
	frontiers := make([]CausalFrontier, 0)
	for _, result := range results {
		if result.CausalFrontier != nil {
			frontiers = append(frontiers, *result.CausalFrontier)
		}
	}
	return frontiers
}

func dominantDecision(vector []Decision) Decision {
	for _, decision := range []Decision{DecisionRefuted, DecisionUnknown, DecisionClosed} {
		for _, value := range vector {
			if value == decision {
				return decision
			}
		}
	}
	return DecisionUnknown
}

func uniqueEventValues(events []TraceEvent, value func(TraceEvent) string) []string {
	seen := make(map[string]bool)
	values := make([]string, 0)
	for _, event := range events {
		item := value(event)
		if !seen[item] {
			seen[item] = true
			values = append(values, item)
		}
	}
	return values
}

func hasStateChange(events []TraceEvent) bool {
	if len(events) < 2 {
		return false
	}
	for index := 1; index < len(events); index++ {
		if events[index].StateIdentity != events[index-1].StateIdentity {
			return true
		}
	}
	return false
}

func detectCycle(visits []StateVisit) (int, int, bool) {
	for start := 0; start < len(visits); start++ {
		for period := 1; start+2*period <= len(visits); period++ {
			matches := true
			for index := start + period; index < len(visits); index++ {
				if visits[index].StateIdentity != visits[start+(index-start)%period].StateIdentity {
					matches = false
					break
				}
			}
			if matches {
				return start, period, true
			}
		}
	}
	return 0, 0, false
}

func fixedPointMatches(trace Trace, spec Spec, evidenceValid bool) bool {
	if trace.FixedPoint == nil || !trace.FixedPoint.Authorized || !evidenceValid || len(trace.Events) == 0 {
		return false
	}
	if trace.FixedPoint.StateIdentity == "" || len(trace.FixedPoint.EvidenceIdentities) != len(spec.Cells) {
		return false
	}
	for _, event := range trace.Events {
		if event.StateIdentity != trace.FixedPoint.StateIdentity {
			return false
		}
		for _, decision := range event.Decisions {
			if trace.FixedPoint.EvidenceIdentities[decision.CellID] != decision.EvidenceIdentity {
				return false
			}
		}
	}
	return true
}

func validateClaims(spec Spec, trace Trace, claims []Claim) map[string]bool {
	status := make(map[string]bool)
	for _, claim := range claims {
		if claim.Kind != "IMPROVEMENT" {
			continue
		}
		if _, ok := status[claim.CellID]; ok {
			continue
		}
		status[claim.CellID] = claimPairMatches(trace, claim)
	}
	_ = spec
	return status
}

func claimPairMatches(trace Trace, claim Claim) bool {
	if claim.Relation != "STRICT_ADVANCE" || claim.FromEvidenceIdentity == "" || claim.ToEvidenceIdentity == "" || claim.FromSequence >= claim.ToSequence {
		return false
	}
	from, fromOK := eventBySequence(trace.Events, claim.FromSequence)
	to, toOK := eventBySequence(trace.Events, claim.ToSequence)
	if !fromOK || !toOK || from.StateIdentity == to.StateIdentity {
		return false
	}
	fromDecision, fromOK := decisionForCell(from, claim.CellID)
	toDecision, toOK := decisionForCell(to, claim.CellID)
	return fromOK && toOK && fromDecision.EvidenceIdentity == claim.FromEvidenceIdentity && toDecision.EvidenceIdentity == claim.ToEvidenceIdentity && fromDecision.EvidenceStateIdentity == from.StateIdentity && toDecision.EvidenceStateIdentity == to.StateIdentity
}

func eventBySequence(events []TraceEvent, sequence int) (TraceEvent, bool) {
	for _, event := range events {
		if event.Sequence == sequence {
			return event, true
		}
	}
	return TraceEvent{}, false
}

func decisionForCell(event TraceEvent, cellID string) (CellDecision, bool) {
	for _, decision := range event.Decisions {
		if decision.CellID == cellID {
			return decision, true
		}
	}
	return CellDecision{}, false
}

func counterexampleOccurrences(trace Trace) map[string][]Counterexample {
	occurrences := make(map[string][]Counterexample)
	for _, event := range trace.Events {
		for _, counterexample := range event.Counterexamples {
			if counterexample.Sequence == 0 {
				counterexample.Sequence = event.Sequence
			}
			if counterexample.StateIdentity == "" {
				counterexample.StateIdentity = event.StateIdentity
			}
			occurrences[counterexample.CellID+"|"+counterexample.Identity] = append(occurrences[counterexample.CellID+"|"+counterexample.Identity], counterexample)
		}
	}
	return occurrences
}

func keyCellID(key string) string {
	for index, value := range key {
		if value == '|' {
			return key[:index]
		}
	}
	return key
}

func EncodeReport(report Report) ([]byte, error) {
	return json.MarshalIndent(report, "", "  ")
}

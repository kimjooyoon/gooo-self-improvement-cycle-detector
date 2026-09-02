package engine

import (
	"encoding/json"
	"fmt"
)

type fixtureDocument struct {
	Name                string                    `json:"name"`
	Events              []fixtureEvent            `json:"events"`
	DecisionDefaults    fixtureDecisionDefaults   `json:"decision_defaults"`
	DecisionOverrides   []fixtureDecisionOverride `json:"decision_overrides"`
	ClaimMode            string                    `json:"claim_mode"`
	Claims              []Claim                   `json:"claims"`
	FixedPoint          *fixtureFixedPoint         `json:"fixed_point,omitempty"`
	Expected            Expected                  `json:"expected"`
}

type fixtureEvent struct {
	Sequence          int              `json:"sequence"`
	StateIdentity     string           `json:"state_identity"`
	StateDigest       string           `json:"state_digest,omitempty"`
	EvaluatorIdentity string           `json:"evaluator_identity"`
	ToolchainIdentity string           `json:"toolchain_identity"`
	ChangeReceipt     ChangeReceipt    `json:"change_receipt"`
	Counterexamples   []Counterexample `json:"counterexamples,omitempty"`
}

type fixtureDecisionDefaults struct {
	Decision       Decision `json:"decision"`
	SemanticEffect string   `json:"semantic_effect"`
	EvidenceMode   string   `json:"evidence_mode"`
}

type fixtureDecisionOverride struct {
	Sequence                int      `json:"sequence"`
	CellID                  string   `json:"cell_id"`
	Decision                Decision `json:"decision,omitempty"`
	SemanticEffect          string   `json:"semantic_effect,omitempty"`
	EvidenceIdentity        string   `json:"evidence_identity,omitempty"`
	EvidenceStateIdentity   string   `json:"evidence_state_identity,omitempty"`
	EvaluatorIdentity       string   `json:"evaluator_identity,omitempty"`
	ToolchainIdentity       string   `json:"toolchain_identity,omitempty"`
	ChangeReceiptIdentity   string   `json:"change_receipt_identity,omitempty"`
}

type fixtureFixedPoint struct {
	Authorized    bool   `json:"authorized"`
	StateIdentity string `json:"state_identity"`
	EvidenceMode  string `json:"evidence_mode"`
}

func LoadFixture(data []byte, authorityJSON []byte) (Fixture, error) {
	var document fixtureDocument
	if err := json.Unmarshal(data, &document); err != nil {
		return Fixture{}, fmt.Errorf("parse fixture: %w", err)
	}
	spec, _, err := ParseSpec(authorityJSON)
	if err != nil {
		return Fixture{}, err
	}
	if document.Name == "" {
		return Fixture{}, fmt.Errorf("fixture name is required")
	}
	if len(document.Events) == 0 {
		return Fixture{}, fmt.Errorf("fixture %s has no events", document.Name)
	}
	trace := Trace{Events: make([]TraceEvent, 0, len(document.Events))}
	for _, source := range document.Events {
		event := TraceEvent{
			Sequence:          source.Sequence,
			StateIdentity:     source.StateIdentity,
			StateDigest:       source.StateDigest,
			EvaluatorIdentity: source.EvaluatorIdentity,
			ToolchainIdentity: source.ToolchainIdentity,
			ChangeReceipt:     source.ChangeReceipt,
			Counterexamples:   append([]Counterexample(nil), source.Counterexamples...),
			Decisions:         make([]CellDecision, 0, len(spec.Cells)),
		}
		for _, cell := range spec.Cells {
			decision := materializeDecision(document.DecisionDefaults, event, cell.ID)
			for _, override := range document.DecisionOverrides {
				if override.Sequence == event.Sequence && override.CellID == cell.ID {
					applyOverride(&decision, override)
				}
			}
			event.Decisions = append(event.Decisions, decision)
		}
		trace.Events = append(trace.Events, event)
	}
	if document.FixedPoint != nil {
		fixed := &FixedPointAuthorization{
			Authorized:    document.FixedPoint.Authorized,
			StateIdentity: document.FixedPoint.StateIdentity,
			EvidenceIdentities: make(map[string]string, len(spec.Cells)),
		}
		if document.FixedPoint.EvidenceMode == "match" {
			last := trace.Events[len(trace.Events)-1]
			for _, decision := range last.Decisions {
				fixed.EvidenceIdentities[decision.CellID] = decision.EvidenceIdentity
			}
		} else if document.FixedPoint.EvidenceMode == "mismatch" {
			last := trace.Events[len(trace.Events)-1]
			for _, decision := range last.Decisions {
				fixed.EvidenceIdentities[decision.CellID] = decision.EvidenceIdentity
			}
			if len(spec.Cells) > 0 {
				fixed.EvidenceIdentities[spec.Cells[0].ID] = "evidence:mismatch"
			}
		}
		trace.FixedPoint = fixed
	}
	claims := append([]Claim(nil), document.Claims...)
	if document.ClaimMode == "exact_first_change" {
		claims = append(claims, exactFirstChangeClaims(spec, trace)...)
	}
	return Fixture{Name: document.Name, Trace: trace, Claims: claims, Expected: document.Expected}, nil
}

func exactFirstChangeClaims(spec Spec, trace Trace) []Claim {
	for fromIndex := 0; fromIndex < len(trace.Events); fromIndex++ {
		for toIndex := fromIndex + 1; toIndex < len(trace.Events); toIndex++ {
			if trace.Events[fromIndex].StateIdentity == trace.Events[toIndex].StateIdentity {
				continue
			}
			claims := make([]Claim, 0, len(spec.Cells))
			for _, cell := range spec.Cells {
				from, fromOK := decisionForCell(trace.Events[fromIndex], cell.ID)
				to, toOK := decisionForCell(trace.Events[toIndex], cell.ID)
				if !fromOK || !toOK {
					continue
				}
				claims = append(claims, Claim{
					CellID: cell.ID, Kind: "IMPROVEMENT", FromSequence: trace.Events[fromIndex].Sequence,
					ToSequence: trace.Events[toIndex].Sequence, FromEvidenceIdentity: from.EvidenceIdentity,
					ToEvidenceIdentity: to.EvidenceIdentity, Relation: "STRICT_ADVANCE",
				})
			}
			return claims
		}
	}
	return nil
}

func materializeDecision(defaults fixtureDecisionDefaults, event TraceEvent, cellID string) CellDecision {
	decision := defaults.Decision
	if decision == "" {
		decision = DecisionClosed
	}
	effect := defaults.SemanticEffect
	if effect == "" {
		effect = "NO_CHANGE"
	}
	evidence := fmt.Sprintf("evidence:%s:%s:%s:%s", event.StateIdentity, cellID, event.EvaluatorIdentity, event.ToolchainIdentity)
	evidenceState := event.StateIdentity
	if defaults.EvidenceMode == "missing" {
		evidence = ""
		evidenceState = ""
	}
	if defaults.EvidenceMode == "stale" {
		evidenceState = "stale-state"
	}
	return CellDecision{
		CellID:                cellID,
		Decision:              decision,
		StateIdentity:         event.StateIdentity,
		EvidenceIdentity:      evidence,
		EvidenceStateIdentity: evidenceState,
		EvaluatorIdentity:     event.EvaluatorIdentity,
		ToolchainIdentity:     event.ToolchainIdentity,
		ChangeReceiptIdentity: event.ChangeReceipt.Identity,
		SemanticEffect:        effect,
		Stage:                 "INGEST",
		Step:                  "materialize",
	}
}

func applyOverride(decision *CellDecision, override fixtureDecisionOverride) {
	if override.Decision != "" {
		decision.Decision = override.Decision
	}
	if override.SemanticEffect != "" {
		decision.SemanticEffect = override.SemanticEffect
	}
	if override.EvidenceIdentity != "" {
		decision.EvidenceIdentity = override.EvidenceIdentity
	}
	if override.EvidenceStateIdentity != "" {
		decision.EvidenceStateIdentity = override.EvidenceStateIdentity
	}
	if override.EvaluatorIdentity != "" {
		decision.EvaluatorIdentity = override.EvaluatorIdentity
	}
	if override.ToolchainIdentity != "" {
		decision.ToolchainIdentity = override.ToolchainIdentity
	}
	if override.ChangeReceiptIdentity != "" {
		decision.ChangeReceiptIdentity = override.ChangeReceiptIdentity
	}
}

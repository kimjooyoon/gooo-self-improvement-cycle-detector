package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

func ParseSpec(authorityJSON []byte) (Spec, AuthorityMetrics, error) {
	var spec Spec
	if err := json.Unmarshal(authorityJSON, &spec); err != nil {
		return Spec{}, AuthorityMetrics{}, fmt.Errorf("parse authority: %w", err)
	}
	if err := ValidateSpec(spec); err != nil {
		return Spec{}, AuthorityMetrics{}, err
	}
	digest := sha256.Sum256(authorityJSON)
	metrics := AuthorityMetrics{
		Schema:            spec.Schema,
		Version:           spec.Version,
		SHA256:            hex.EncodeToString(digest[:]),
		ActivityCount:     len(spec.Activities),
		CellCount:         len(spec.Cells),
		ProofChoiceCounts: copyCounts(spec.ProofChoices),
		IndicatorCounts:   copyCounts(spec.Indicators),
		RequiredEvidence:  append([]string(nil), spec.RequiredEvidence...),
		NoGlobalScore:     boolRule(spec.Rules, "no_global_score"),
	}
	return spec, metrics, nil
}

func ValidateSpec(spec Spec) error {
	if spec.Schema == "" || spec.Version < 1 {
		return fmt.Errorf("authority schema and version are required")
	}
	if len(spec.Cells) != 12 || len(spec.Activities) != 12 {
		return fmt.Errorf("authority requires exactly 12 cells and 12 activities")
	}
	if len(spec.DecisionPrecedence) != 3 || spec.DecisionPrecedence[0] != DecisionRefuted || spec.DecisionPrecedence[1] != DecisionUnknown || spec.DecisionPrecedence[2] != DecisionClosed {
		return fmt.Errorf("authority decision precedence must be REFUTED, UNKNOWN, CLOSED")
	}
	if spec.ProofChoices["FOUNDATION"] != 4 || spec.ProofChoices["COHERENCE"] != 4 || spec.ProofChoices["REGRESSION"] != 4 {
		return fmt.Errorf("authority proof choices must be 4/4/4")
	}
	if spec.Indicators["DRIVER"] != 4 || spec.Indicators["OUTCOME"] != 4 || spec.Indicators["GUARDRAIL"] != 4 {
		return fmt.Errorf("authority indicators must be 4/4/4")
	}
	seenCells := make(map[string]bool, len(spec.Cells))
	seenActivities := make(map[string]bool, len(spec.Activities))
	proofCounts := make(map[string]int)
	indicatorCounts := make(map[string]int)
	for _, activity := range spec.Activities {
		if activity.ID == "" || seenActivities[activity.ID] {
			return fmt.Errorf("authority activity ids must be unique and non-empty")
		}
		seenActivities[activity.ID] = true
	}
	for _, cell := range spec.Cells {
		if cell.ID == "" || seenCells[cell.ID] {
			return fmt.Errorf("authority cell ids must be unique and non-empty")
		}
		if !seenActivities[cell.Activity] {
			return fmt.Errorf("cell %s references unknown activity %s", cell.ID, cell.Activity)
		}
		seenCells[cell.ID] = true
		proofCounts[cell.ProofChoice]++
		indicatorCounts[cell.Indicator]++
	}
	if !sameCounts(proofCounts, spec.ProofChoices) || !sameCounts(indicatorCounts, spec.Indicators) {
		return fmt.Errorf("cell distributions do not match declared authority counts")
	}
	if len(spec.RequiredEvidence) == 0 || len(spec.UnknownClasses) == 0 || len(spec.Stages) == 0 {
		return fmt.Errorf("authority evidence, unknown classes, and stages are required")
	}
	return nil
}

func copyCounts(input map[string]int) map[string]int {
	output := make(map[string]int, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func sameCounts(left, right map[string]int) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if right[key] != value {
			return false
		}
	}
	return true
}

func boolRule(rules map[string]any, key string) bool {
	value, ok := rules[key].(bool)
	return ok && value
}

package engine

import (
	"fmt"
	"reflect"
)

func VerifyExpected(report Report, expected Expected) error {
	if len(expected.DecisionVector) > 0 && !reflect.DeepEqual(expected.DecisionVector, report.DecisionVector) {
		return fmt.Errorf("decision vector mismatch: got %v want %v", report.DecisionVector, expected.DecisionVector)
	}
	if expected.DetectionKindsExpected {
		got := make([]string, 0, len(report.Detections))
		for _, detection := range report.Detections {
			got = append(got, detection.Kind)
		}
		if !reflect.DeepEqual(expected.DetectionKinds, got) {
			return fmt.Errorf("detection kinds mismatch: got %v want %v", got, expected.DetectionKinds)
		}
	}
	if expected.PeriodsExpected {
		got := make([]int, 0)
		for _, detection := range report.Detections {
			if detection.Period > 0 {
				got = append(got, detection.Period)
			}
		}
		if !reflect.DeepEqual(expected.Periods, got) {
			return fmt.Errorf("period mismatch: got %v want %v", got, expected.Periods)
		}
	}
	if expected.FrontierSequencesExpected {
		got := make([]int, 0, len(report.CausalFrontier))
		for _, frontier := range report.CausalFrontier {
			got = append(got, frontier.Sequence)
		}
		if !reflect.DeepEqual(expected.FrontierSequences, got) {
			return fmt.Errorf("frontier sequence mismatch: got %v want %v", got, expected.FrontierSequences)
		}
	}
	if len(report.OperationalRefuted) != expected.OperationalRefutedCount {
		return fmt.Errorf("operational refuted count mismatch: got %d want %d", len(report.OperationalRefuted), expected.OperationalRefutedCount)
	}
	return nil
}

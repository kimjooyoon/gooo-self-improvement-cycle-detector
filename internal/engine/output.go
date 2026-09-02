package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type CaseSummary struct {
	Name                  string       `json:"name"`
	CaseDecision          Decision     `json:"case_decision"`
	DecisionVector        []Decision   `json:"decision_vector"`
	DetectionKinds        []string     `json:"detection_kinds"`
	Periods               []int        `json:"periods"`
	FrontierSequences     []int        `json:"frontier_sequences"`
	OperationalRefuted   []OperationalEvent `json:"operational_refuted,omitempty"`
}

type ConformanceReport struct {
	Schema            string            `json:"schema"`
	CaseCount         int               `json:"case_count"`
	CaseCounts        map[string]int    `json:"case_counts"`
	DecisionCounts    map[string]int    `json:"decision_counts"`
	Cases             []CaseSummary     `json:"cases"`
	Authority         AuthorityMetrics  `json:"authority"`
	Inventory         InventoryMetrics  `json:"inventory"`
	Metadata          RunMetadata       `json:"metadata,omitempty"`
}

type ReplayReport struct {
	Schema             string `json:"schema"`
	CaseCount          int    `json:"case_count"`
	Deterministic      bool   `json:"deterministic"`
	FirstDigest        string `json:"first_digest"`
	SecondDigest       string `json:"second_digest"`
}

func WriteReport(outDir string, report Report) error {
	return WriteJSON(outDir, "report.json", report)
}

func WriteJSON(outDir, name string, value any) error {
	if filepath.Base(name) != name || strings.Contains(name, string(filepath.Separator)) {
		return fmt.Errorf("output name must be a file name: %s", name)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", name, err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(filepath.Join(outDir, name), data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}
	return nil
}

func BuildConformanceReport(authorityJSON []byte, fixtures []Fixture, reports []Report, inventory InventoryMetrics, metadata RunMetadata) (ConformanceReport, error) {
	_, authority, err := ParseSpec(authorityJSON)
	if err != nil {
		return ConformanceReport{}, err
	}
	if len(fixtures) != len(reports) {
		return ConformanceReport{}, fmt.Errorf("fixture/report count mismatch")
	}
	summary := ConformanceReport{
		Schema:         "gooo.trace.conformance.report/v1",
		CaseCount:      len(reports),
		CaseCounts:     map[string]int{},
		DecisionCounts: map[string]int{},
		Cases:          make([]CaseSummary, 0, len(reports)),
		Authority:      authority,
		Inventory:      inventory,
		Metadata:       metadata,
	}
	for index, report := range reports {
		if fixtures[index].Name != report.CaseName {
			return ConformanceReport{}, fmt.Errorf("report name mismatch for fixture %d", index)
		}
		caseSummary := CaseSummary{
			Name:                report.CaseName,
			CaseDecision:        report.CaseDecision,
			DecisionVector:      append([]Decision(nil), report.DecisionVector...),
			DetectionKinds:      make([]string, 0, len(report.Detections)),
			Periods:             make([]int, 0),
			FrontierSequences:   make([]int, 0, len(report.CausalFrontier)),
			OperationalRefuted:  append([]OperationalEvent(nil), report.OperationalRefuted...),
		}
		for _, detection := range report.Detections {
			caseSummary.DetectionKinds = append(caseSummary.DetectionKinds, detection.Kind)
			if detection.Period > 0 {
				caseSummary.Periods = append(caseSummary.Periods, detection.Period)
			}
		}
		for _, frontier := range report.CausalFrontier {
			caseSummary.FrontierSequences = append(caseSummary.FrontierSequences, frontier.Sequence)
		}
		sort.Ints(caseSummary.FrontierSequences)
		summary.Cases = append(summary.Cases, caseSummary)
		summary.CaseCounts[string(report.CaseDecision)]++
		for _, decision := range report.DecisionVector {
			summary.DecisionCounts[string(decision)]++
		}
	}
	return summary, nil
}

func BuildInventory(root string) (InventoryMetrics, error) {
	var included []string
	var excluded []string
	var authority []string
	var generated []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}
		if entry.IsDir() {
			if relative == ".git" || strings.HasPrefix(relative, ".git"+string(filepath.Separator)) || relative == "work" || relative == "outputs" {
				return filepath.SkipDir
			}
			return nil
		}
		if relative == "README.md" {
			excluded = append(excluded, relative)
			return nil
		}
		included = append(included, relative)
		if relative == filepath.Join(".gooo", "semantics.gooo") {
			authority = append(authority, relative)
		}
		if strings.HasPrefix(relative, filepath.Join("internal", "generated")+string(filepath.Separator)) {
			generated = append(generated, relative)
		}
		return nil
	})
	if err != nil {
		return InventoryMetrics{}, fmt.Errorf("inventory %s: %w", root, err)
	}
	sort.Strings(included)
	sort.Strings(excluded)
	sort.Strings(authority)
	sort.Strings(generated)
	return InventoryMetrics{IncludedFileCount: len(included), ExcludedFiles: excluded, AuthorityFiles: authority, GeneratedFiles: generated}, nil
}

func DigestJSON(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}

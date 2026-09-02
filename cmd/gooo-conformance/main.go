package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kimjooyoon/gooo-self-improvement-cycle-detector/internal/engine"
	"github.com/kimjooyoon/gooo-self-improvement-cycle-detector/internal/generated"
)

func main() {
	fixtureDir := flag.String("fixtures", "fixtures", "deterministic fixture directory")
	outDir := flag.String("out", "", "caller-owned output directory")
	repoRoot := flag.String("repo-root", ".", "repository root for inventory")
	flag.Parse()
	if *outDir == "" {
		fatal("--out is required", nil)
	}
	entries, err := os.ReadDir(*fixtureDir)
	if err != nil {
		fatal("read fixtures", err)
	}
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)
	fixtures := make([]engine.Fixture, 0, len(files))
	reports := make([]engine.Report, 0, len(files))
	caseDir := filepath.Join(*outDir, "cases")
	for _, file := range files {
		path := filepath.Join(*fixtureDir, file)
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			fatal("read "+path, readErr)
		}
		fixture, loadErr := generated.LoadFixture(data)
		if loadErr != nil {
			fatal("load "+path, loadErr)
		}
		report, analyzeErr := generated.Analyze(fixture.Name, fixture.Trace, fixture.Claims)
		if analyzeErr != nil {
			fatal("analyze "+fixture.Name, analyzeErr)
		}
		if verifyErr := engine.VerifyExpected(report, fixture.Expected); verifyErr != nil {
			fatal("verify "+fixture.Name, verifyErr)
		}
		fixtures = append(fixtures, fixture)
		reports = append(reports, report)
		if writeErr := engine.WriteJSON(caseDir, fixture.Name+".json", report); writeErr != nil {
			fatal("write case "+fixture.Name, writeErr)
		}
	}
	inventory, err := engine.BuildInventory(*repoRoot)
	if err != nil {
		fatal("build inventory", err)
	}
	metadata := runMetadata()
	summary, err := generated.BuildConformanceReport(fixtures, reports, inventory, metadata)
	if err != nil {
		fatal("build conformance report", err)
	}
	if err := engine.WriteJSON(*outDir, "conformance-report.json", summary); err != nil {
		fatal("write conformance report", err)
	}
	if err := engine.WriteJSON(*outDir, "inventory.json", inventory); err != nil {
		fatal("write inventory", err)
	}
	if err := engine.WriteJSON(*outDir, "authority.json", summary.Authority); err != nil {
		fatal("write authority", err)
	}
	fmt.Printf("validated %d deterministic fixtures\n", len(reports))
}

func runMetadata() engine.RunMetadata {
	return engine.RunMetadata{
		Repository: os.Getenv("GITHUB_REPOSITORY"),
		CommitSHA:  os.Getenv("GITHUB_SHA"),
		RunID:      os.Getenv("GITHUB_RUN_ID"),
		RunAttempt: os.Getenv("GITHUB_RUN_ATTEMPT"),
		Workflow:   os.Getenv("GITHUB_WORKFLOW"),
		Job:        os.Getenv("GITHUB_JOB"),
	}
}

func fatal(action string, err error) {
	if err == nil {
		panic(action)
	}
	fmt.Fprintf(os.Stderr, "gooo-conformance: %s: %v\n", action, err)
	os.Exit(1)
}

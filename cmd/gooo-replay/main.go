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
	first := make([]engine.Report, 0, len(files))
	second := make([]engine.Report, 0, len(files))
	for _, file := range files {
		data, readErr := os.ReadFile(filepath.Join(*fixtureDir, file))
		if readErr != nil {
			fatal("read "+file, readErr)
		}
		fixture, loadErr := generated.LoadFixture(data)
		if loadErr != nil {
			fatal("load "+file, loadErr)
		}
		firstReport, firstErr := generated.Analyze(fixture.Name, fixture.Trace, fixture.Claims)
		if firstErr != nil {
			fatal("first replay "+fixture.Name, firstErr)
		}
		secondReport, secondErr := generated.Analyze(fixture.Name, fixture.Trace, fixture.Claims)
		if secondErr != nil {
			fatal("second replay "+fixture.Name, secondErr)
		}
		first = append(first, firstReport)
		second = append(second, secondReport)
	}
	firstDigest, err := engine.DigestJSON(first)
	if err != nil {
		fatal("digest first replay", err)
	}
	secondDigest, err := engine.DigestJSON(second)
	if err != nil {
		fatal("digest second replay", err)
	}
	report := engine.ReplayReport{Schema: "gooo.trace.replay.report/v1", CaseCount: len(first), Deterministic: firstDigest == secondDigest, FirstDigest: firstDigest, SecondDigest: secondDigest}
	if err := engine.WriteJSON(*outDir, "replay-report.json", report); err != nil {
		fatal("write replay report", err)
	}
	if !report.Deterministic {
		fatal("replay is not deterministic", nil)
	}
	fmt.Printf("replayed %d deterministic fixtures\n", len(first))
}

func fatal(action string, err error) {
	if err == nil {
		panic(action)
	}
	fmt.Fprintf(os.Stderr, "gooo-replay: %s: %v\n", action, err)
	os.Exit(1)
}

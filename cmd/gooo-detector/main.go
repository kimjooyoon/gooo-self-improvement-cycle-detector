package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/kimjooyoon/gooo-self-improvement-cycle-detector/internal/engine"
	"github.com/kimjooyoon/gooo-self-improvement-cycle-detector/internal/generated"
)

type traceDocument struct {
	Name   string        `json:"name"`
	Trace  engine.Trace  `json:"trace"`
	Claims []engine.Claim `json:"claims"`
}

func main() {
	tracePath := flag.String("trace", "", "trace or compact fixture JSON")
	outDir := flag.String("out", "", "caller-owned output directory")
	flag.Parse()
	if *tracePath == "" || *outDir == "" {
		fatal("--trace and --out are required")
	}
	data, err := os.ReadFile(*tracePath)
	if err != nil {
		fatal("read trace", err)
	}
	var document traceDocument
	if err := json.Unmarshal(data, &document); err != nil {
		fatal("parse trace", err)
	}
	var report engine.Report
	if len(document.Trace.Events) > 0 {
		report, err = generated.Analyze(document.Name, document.Trace, document.Claims)
	} else {
		fixture, loadErr := generated.LoadFixture(data)
		err = loadErr
		if err == nil {
			report, err = generated.Analyze(fixture.Name, fixture.Trace, fixture.Claims)
		}
	}
	if err != nil {
		fatal("analyze trace", err)
	}
	if err := generated.WriteReport(*outDir, report); err != nil {
		fatal("write report", err)
	}
}

func fatal(action string, err error) {
	fmt.Fprintf(os.Stderr, "gooo-detector: %s: %v\n", action, err)
	os.Exit(1)
}

//go:generate go run . -authority ../../.gooo/semantics.gooo -out ../../internal/generated

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/kimjooyoon/gooo-self-improvement-cycle-detector/internal/engine"
)

func main() {
	authorityPath := flag.String("authority", ".gooo/semantics.gooo", "path to the authoritative .gooo metacode")
	outputDir := flag.String("out", "internal/generated", "directory for generated Go")
	flag.Parse()
	data, err := os.ReadFile(*authorityPath)
	if err != nil {
		fatal("read authority", err)
	}
	if _, _, err := engine.ParseSpec(data); err != nil {
		fatal("validate authority", err)
	}
	digest := sha256.Sum256(data)
	if err := os.MkdirAll(*outputDir, 0o755); err != nil {
		fatal("create generated directory", err)
	}
	source := fmt.Sprintf(`// Code generated from .gooo/semantics.gooo; DO NOT EDIT.
package generated

import "github.com/kimjooyoon/gooo-self-improvement-cycle-detector/internal/engine"

const AuthoritySHA256 = %s

var AuthorityJSON = []byte(%s)

func Analyze(name string, trace engine.Trace, claims []engine.Claim) (engine.Report, error) {
	return engine.Analyze(AuthorityJSON, name, trace, claims)
}

func LoadFixture(data []byte) (engine.Fixture, error) {
	return engine.LoadFixture(data, AuthorityJSON)
}

func WriteReport(outDir string, report engine.Report) error {
	return engine.WriteReport(outDir, report)
}

func BuildConformanceReport(fixtures []engine.Fixture, reports []engine.Report, inventory engine.InventoryMetrics, metadata engine.RunMetadata) (engine.ConformanceReport, error) {
	return engine.BuildConformanceReport(AuthorityJSON, fixtures, reports, inventory, metadata)
}
`, strconv.Quote(hex.EncodeToString(digest[:])), strconv.Quote(string(data)))
	outputPath := filepath.Join(*outputDir, "detector_gen.go")
	if err := os.WriteFile(outputPath, []byte(source), 0o644); err != nil {
		fatal("write generated detector", err)
	}
}

func fatal(action string, err error) {
	fmt.Fprintf(os.Stderr, "gooo-gen: %s: %v\n", action, err)
	os.Exit(1)
}

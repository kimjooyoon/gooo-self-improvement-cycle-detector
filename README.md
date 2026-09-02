# gooo-self-improvement-cycle-detector

This repository contains a trace detector for the Gooo self-improvement loop.

The semantic authority is [`semantics.gooo`](.gooo/semantics.gooo). It defines the
12 activities, 12 conformance cells, evidence identities, decision precedence, and
the rules that prohibit global scores or progress inferred from hashes, line counts,
or commit counts. Go is generated from that metacode in GitHub Actions.

The detector consumes an append-only trace containing immutable semantic-state
identities, per-cell evidence, evaluator and toolchain identities, and change
receipts. It returns per-cell decision vectors and the minimal causal frontier.
`REFUTED` dominates `UNKNOWN`, which dominates `CLOSED`. An unknown record always
includes its stage, step, reason, unknown class, next operation, and blocker.

Conformance fixtures cover bounded advance, explicit fixed points, repeated no-op,
period-2 oscillation, period-N cycles, regression, evaluator drift, missing state,
stale evidence, ambiguous ordering, counterexample recurrence, and a hash-identical
trace with missing evidence. Runtime output is written only to a caller-owned
directory.

All generation, formatting, `go fix`, build, vet, tests, replay, RSS observation,
conformance, and report creation run in GitHub Actions. The root README is excluded
from the generated inventory.

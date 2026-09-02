package engine

type Decision string

const (
	DecisionRefuted Decision = "REFUTED"
	DecisionUnknown Decision = "UNKNOWN"
	DecisionClosed  Decision = "CLOSED"
)

type CellSpec struct {
	ID          string `json:"id"`
	ProofChoice string `json:"proof_choice"`
	Indicator   string `json:"indicator"`
	Activity    string `json:"activity"`
}

type Spec struct {
	Schema             string            `json:"schema"`
	Version            int               `json:"version"`
	DecisionPrecedence []Decision        `json:"decision_precedence"`
	ProofChoices       map[string]int    `json:"proof_choices"`
	Indicators         map[string]int    `json:"indicators"`
	Activities         []CellSpec        `json:"activities"`
	Cells              []CellSpec        `json:"cells"`
	RequiredEvidence   []string          `json:"required_evidence"`
	Stages             []string          `json:"stages"`
	UnknownClasses     []string          `json:"unknown_classes"`
	Rules              map[string]any    `json:"rules"`
}

type Trace struct {
	Events     []TraceEvent              `json:"events"`
	FixedPoint *FixedPointAuthorization   `json:"fixed_point,omitempty"`
}

type TraceEvent struct {
	Sequence           int              `json:"sequence"`
	StateIdentity      string           `json:"state_identity"`
	StateDigest        string           `json:"state_digest,omitempty"`
	EvaluatorIdentity  string           `json:"evaluator_identity"`
	ToolchainIdentity  string           `json:"toolchain_identity"`
	ChangeReceipt      ChangeReceipt    `json:"change_receipt"`
	Decisions          []CellDecision   `json:"decisions"`
	Counterexamples    []Counterexample `json:"counterexamples,omitempty"`
}

type ChangeReceipt struct {
	Identity             string `json:"identity"`
	Operation            string `json:"operation"`
	ParentStateIdentity  string `json:"parent_state_identity,omitempty"`
	ResultStateIdentity  string `json:"result_state_identity,omitempty"`
	Authorized           bool   `json:"authorized,omitempty"`
}

type CellDecision struct {
	CellID                   string   `json:"cell_id"`
	Decision                 Decision `json:"decision"`
	StateIdentity            string   `json:"state_identity"`
	EvidenceIdentity         string   `json:"evidence_identity"`
	EvidenceStateIdentity    string   `json:"evidence_state_identity"`
	EvaluatorIdentity        string   `json:"evaluator_identity"`
	ToolchainIdentity        string   `json:"toolchain_identity"`
	ChangeReceiptIdentity    string   `json:"change_receipt_identity"`
	SemanticEffect           string   `json:"semantic_effect"`
	Stage                    string   `json:"stage,omitempty"`
	Step                     string   `json:"step,omitempty"`
	Reason                   string   `json:"reason,omitempty"`
	UnknownClass             string   `json:"unknown_class,omitempty"`
	NextOperation            string   `json:"next_operation,omitempty"`
	BlockedBy                string   `json:"blocked_by,omitempty"`
}

type Counterexample struct {
	Identity          string `json:"identity"`
	CellID            string `json:"cell_id"`
	EvidenceIdentity  string `json:"evidence_identity"`
	StateIdentity     string `json:"state_identity"`
	Sequence          int    `json:"sequence"`
}

type Claim struct {
	CellID                 string `json:"cell_id"`
	Kind                   string `json:"kind"`
	FromSequence           int    `json:"from_sequence"`
	ToSequence             int    `json:"to_sequence"`
	FromEvidenceIdentity   string `json:"from_evidence_identity"`
	ToEvidenceIdentity     string `json:"to_evidence_identity"`
	Relation               string `json:"relation"`
}

type FixedPointAuthorization struct {
	Authorized          bool              `json:"authorized"`
	StateIdentity       string            `json:"state_identity"`
	EvidenceIdentities  map[string]string `json:"evidence_identities"`
}

type UnknownRecord struct {
	Stage         string `json:"stage"`
	Step          string `json:"step"`
	Reason        string `json:"reason"`
	UnknownClass  string `json:"unknown_class"`
	NextOperation string `json:"next_operation"`
	BlockedBy     string `json:"blocked_by"`
}

type Refutation struct {
	Kind              string `json:"kind"`
	Reason            string `json:"reason"`
	CounterexampleIDs []string `json:"counterexample_ids,omitempty"`
}

type StateVisit struct {
	Sequence      int    `json:"sequence"`
	StateIdentity string `json:"state_identity"`
}

type CausalFrontier struct {
	CellID             string      `json:"cell_id"`
	Sequence           int         `json:"sequence"`
	StateIdentity      string      `json:"state_identity"`
	Cause              string      `json:"cause"`
	EvidenceIdentity   string      `json:"evidence_identity,omitempty"`
	RepeatedStatePath  []StateVisit `json:"repeated_state_path,omitempty"`
}

type Detection struct {
	Kind                  string       `json:"kind"`
	Period                int          `json:"period,omitempty"`
	CycleStartSequence    int          `json:"cycle_start_sequence,omitempty"`
	RepeatedStatePath     []StateVisit `json:"repeated_state_path,omitempty"`
	MinimalCausalFrontier []CausalFrontier `json:"minimal_causal_frontier,omitempty"`
}

type CellResult struct {
	CellID          string          `json:"cell_id"`
	ProofChoice     string          `json:"proof_choice"`
	Indicator       string          `json:"indicator"`
	Activity        string          `json:"activity"`
	Decision        Decision        `json:"decision"`
	Unknown         *UnknownRecord  `json:"unknown,omitempty"`
	Refutation      *Refutation     `json:"refutation,omitempty"`
	CausalFrontier  *CausalFrontier `json:"causal_frontier,omitempty"`
}

type RunMetadata struct {
	Repository string `json:"repository,omitempty"`
	CommitSHA  string `json:"commit_sha,omitempty"`
	RunID      string `json:"run_id,omitempty"`
	RunAttempt string `json:"run_attempt,omitempty"`
	Workflow   string `json:"workflow,omitempty"`
	Job        string `json:"job,omitempty"`
}

type AuthorityMetrics struct {
	Schema             string         `json:"schema"`
	Version            int            `json:"version"`
	SHA256             string         `json:"sha256"`
	ActivityCount      int            `json:"activity_count"`
	CellCount          int            `json:"cell_count"`
	ProofChoiceCounts  map[string]int `json:"proof_choice_counts"`
	IndicatorCounts    map[string]int `json:"indicator_counts"`
	RequiredEvidence   []string       `json:"required_evidence"`
	NoGlobalScore      bool           `json:"no_global_score"`
}

type InventoryMetrics struct {
	IncludedFileCount int      `json:"included_file_count"`
	ExcludedFiles     []string `json:"excluded_files"`
	AuthorityFiles    []string `json:"authority_files"`
	GeneratedFiles    []string `json:"generated_files"`
}

type Report struct {
	Schema             string            `json:"schema"`
	CaseName           string            `json:"case_name"`
	CaseDecision       Decision          `json:"case_decision"`
	DecisionVector     []Decision        `json:"decision_vector"`
	Cells              []CellResult      `json:"cells"`
	Detections         []Detection       `json:"detections"`
	CausalFrontier     []CausalFrontier  `json:"causal_frontier"`
	OperationalRefuted []OperationalEvent `json:"operational_refuted,omitempty"`
	Authority          AuthorityMetrics  `json:"authority"`
	Metadata           RunMetadata       `json:"metadata,omitempty"`
}

type OperationalEvent struct {
	Event             string `json:"event"`
	CellID            string `json:"cell_id"`
	Sequence          int    `json:"sequence"`
	StateIdentity     string `json:"state_identity"`
	Reason            string `json:"reason"`
	NextOperation     string `json:"next_operation"`
}

type Expected struct {
	DecisionVector             []Decision `json:"decision_vector"`
	DetectionKinds             []string   `json:"detection_kinds"`
	DetectionKindsExpected     bool       `json:"detection_kinds_expected"`
	Periods                    []int      `json:"periods"`
	PeriodsExpected            bool       `json:"periods_expected"`
	FrontierSequences          []int      `json:"frontier_sequences"`
	FrontierSequencesExpected  bool       `json:"frontier_sequences_expected"`
	OperationalRefutedCount    int        `json:"operational_refuted_count"`
}

type Fixture struct {
	Name     string   `json:"name"`
	Trace    Trace    `json:"trace"`
	Claims   []Claim  `json:"claims"`
	Expected Expected `json:"expected"`
}

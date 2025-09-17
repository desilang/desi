package lexbridge

type Level string

const (
	LevelError   Level = "error"
	LevelWarning Level = "warning"
	LevelNote    Level = "note"
)

type Applicability string

const (
	MachineApplicable Applicability = "machine-applicable"
	MaybeIncorrect    Applicability = "maybe-incorrect"
	HasPlaceholders   Applicability = "has-placeholders"
)

type Span struct {
	File    string
	Line    int
	Col     int
	EndCol  int // exclusive; 0 => single-col
	Label   string
	Primary bool
}

type Suggestion struct {
	At            Span
	Replacement   string
	Message       string
	Applicability Applicability
}

type Diagnostic struct {
	Level   Level
	Code    string
	Message string

	Primary     Span
	Secondaries []Span

	Notes   []string
	Help    string
	Suggest []Suggestion
}

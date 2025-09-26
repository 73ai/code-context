package output

import (
	"context"
	"io"
	"time"
)

// OutputFormat represents the output format type
type OutputFormat string

const (
	FormatText  OutputFormat = "text"
	FormatJSON  OutputFormat = "json"
	FormatCount OutputFormat = "count"
	FormatFiles OutputFormat = "files"
)

// OutputMode represents different output modes
type OutputMode string

const (
	ModeDefault       OutputMode = "default"       // Show matches with context
	ModeFilesOnly     OutputMode = "files-only"    // Only show filenames with matches
	ModeCount         OutputMode = "count"         // Only show match counts
	ModeOnlyMatching  OutputMode = "only-matching" // Show only the matching part
)

// Match represents a single search match
type Match struct {
	// File information
	Path     string `json:"path"`
	Language string `json:"language,omitempty"`

	// Match location
	LineNumber      int    `json:"line_number"`
	ColumnNumber    int    `json:"column_number,omitempty"`
	AbsoluteOffset  int64  `json:"absolute_offset"`

	// Match content
	Line        string      `json:"line"`
	Submatches  []Submatch  `json:"submatches"`

	// Context lines
	BeforeContext []ContextLine `json:"before_context,omitempty"`
	AfterContext  []ContextLine `json:"after_context,omitempty"`

	// Semantic metadata (extensions beyond ripgrep)
	Semantic *SemanticInfo `json:"semantic,omitempty"`
}

// Submatch represents a portion of a line that matched
type Submatch struct {
	Text  string `json:"text"`
	Start int    `json:"start"`
	End   int    `json:"end"`
}

// ContextLine represents a context line around a match
type ContextLine struct {
	LineNumber int    `json:"line_number"`
	Text       string `json:"text"`
}

// SemanticInfo contains semantic information about matches
type SemanticInfo struct {
	SymbolType   string `json:"symbol_type,omitempty"`   // function, variable, class, etc.
	SymbolName   string `json:"symbol_name,omitempty"`
	Scope        string `json:"scope,omitempty"`         // namespace, class, function
	Definition   *Location `json:"definition,omitempty"` // where this symbol is defined
	References   []Location `json:"references,omitempty"` // other references to this symbol
}

// Location represents a code location
type Location struct {
	Path         string `json:"path"`
	LineNumber   int    `json:"line_number"`
	ColumnNumber int    `json:"column_number"`
}

// FileResult represents results for a single file
type FileResult struct {
	Path         string    `json:"path"`
	Matches      []Match   `json:"matches,omitempty"`
	MatchCount   int       `json:"match_count"`
	BinaryOffset *int64    `json:"binary_offset,omitempty"`
	Stats        FileStats `json:"stats"`
}

// FileStats contains statistics about search results in a file
type FileStats struct {
	Elapsed        Duration `json:"elapsed"`
	Searches       int      `json:"searches"`
	SearchesWithMatch int   `json:"searches_with_match"`
	BytesSearched  int64    `json:"bytes_searched"`
	BytesPrinted   int64    `json:"bytes_printed"`
	MatchedLines   int      `json:"matched_lines"`
	Matches        int      `json:"matches"`
}

// SearchSummary contains overall search statistics
type SearchSummary struct {
	ElapsedTotal Duration         `json:"elapsed_total"`
	Stats        FileStats        `json:"stats"`
	Files        map[string]int   `json:"files,omitempty"` // file extension -> count
}

// Duration represents a time duration compatible with ripgrep's format
type Duration struct {
	Secs  int64  `json:"secs"`
	Nanos int64  `json:"nanos"`
	Human string `json:"human"`
}

// NewDuration creates a Duration from time.Duration
func NewDuration(d time.Duration) Duration {
	nanos := d.Nanoseconds()
	secs := nanos / 1e9
	remainingNanos := nanos % 1e9

	return Duration{
		Secs:  secs,
		Nanos: remainingNanos,
		Human: d.String(),
	}
}

// FormatterConfig contains configuration for output formatting
type FormatterConfig struct {
	Format          OutputFormat
	Mode            OutputMode
	ShowLineNumbers bool
	ShowFilenames   bool
	ShowColors      bool
	MaxColumns      int
	ContextBefore   int
	ContextAfter    int
	OnlyMatching    bool

	// File processing context
	TotalFiles      int // Total number of files being processed

	// Semantic extensions
	IncludeSemantic bool
	ShowDefinitions bool
	ShowReferences  bool
}

// Formatter defines the interface for output formatting
type Formatter interface {
	// Format a single match
	FormatMatch(match Match) error

	// Format file begin marker (for JSON streaming)
	FormatFileBegin(path string) error

	// Format file end marker with stats
	FormatFileEnd(result FileResult) error

	// Format search summary
	FormatSummary(summary SearchSummary) error

	// Flush any buffered output
	Flush() error

	// Close the formatter
	Close() error
}

// FormatterFactory creates formatters based on configuration
type FormatterFactory struct {
	writer io.Writer
	config FormatterConfig
}

// NewFormatterFactory creates a new formatter factory
func NewFormatterFactory(writer io.Writer, config FormatterConfig) *FormatterFactory {
	return &FormatterFactory{
		writer: writer,
		config: config,
	}
}

// CreateFormatter creates a formatter based on the configuration
func (f *FormatterFactory) CreateFormatter() Formatter {
	switch f.config.Format {
	case FormatJSON:
		return NewJSONFormatter(f.writer, f.config)
	case FormatText:
		return NewTextFormatter(f.writer, f.config)
	case FormatCount:
		return NewCountFormatter(f.writer, f.config)
	case FormatFiles:
		return NewFilesFormatter(f.writer, f.config)
	default:
		return NewTextFormatter(f.writer, f.config)
	}
}

// OutputManager manages the overall output process
type OutputManager struct {
	formatter Formatter
	ctx       context.Context
}

// NewOutputManager creates a new output manager
func NewOutputManager(ctx context.Context, formatter Formatter) *OutputManager {
	return &OutputManager{
		formatter: formatter,
		ctx:       ctx,
	}
}

// ProcessMatch processes a single match through the formatter
func (om *OutputManager) ProcessMatch(match Match) error {
	select {
	case <-om.ctx.Done():
		return om.ctx.Err()
	default:
		return om.formatter.FormatMatch(match)
	}
}

// ProcessFileBegin processes the beginning of a file
func (om *OutputManager) ProcessFileBegin(path string) error {
	select {
	case <-om.ctx.Done():
		return om.ctx.Err()
	default:
		return om.formatter.FormatFileBegin(path)
	}
}

// ProcessFileEnd processes the end of a file with results
func (om *OutputManager) ProcessFileEnd(result FileResult) error {
	select {
	case <-om.ctx.Done():
		return om.ctx.Err()
	default:
		return om.formatter.FormatFileEnd(result)
	}
}

// ProcessSummary processes the final search summary
func (om *OutputManager) ProcessSummary(summary SearchSummary) error {
	select {
	case <-om.ctx.Done():
		return om.ctx.Err()
	default:
		return om.formatter.FormatSummary(summary)
	}
}

// Close closes the output manager
func (om *OutputManager) Close() error {
	return om.formatter.Close()
}
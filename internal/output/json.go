package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// JSONFormatter implements ripgrep-compatible JSON output
type JSONFormatter struct {
	writer  io.Writer
	config  FormatterConfig
	encoder *json.Encoder
	stats   map[string]*FileStats
}

func NewJSONFormatter(writer io.Writer, config FormatterConfig) *JSONFormatter {
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)

	return &JSONFormatter{
		writer:  writer,
		config:  config,
		encoder: encoder,
		stats:   make(map[string]*FileStats),
	}
}

// JSONMessage represents the structure of ripgrep's JSON messages
type JSONMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// JSONBeginData represents the data for a "begin" message
type JSONBeginData struct {
	Path JSONPath `json:"path"`
}

// JSONEndData represents the data for an "end" message
type JSONEndData struct {
	Path         JSONPath  `json:"path"`
	BinaryOffset *int64    `json:"binary_offset"`
	Stats        FileStats `json:"stats"`
}

// JSONMatchData represents the data for a "match" message
type JSONMatchData struct {
	Path           JSONPath     `json:"path"`
	Lines          JSONText     `json:"lines"`
	LineNumber     int          `json:"line_number"`
	AbsoluteOffset int64        `json:"absolute_offset"`
	Submatches     []JSONSubmatch `json:"submatches"`
}

// JSONSummaryData represents the data for a "summary" message
type JSONSummaryData struct {
	ElapsedTotal Duration  `json:"elapsed_total"`
	Stats        FileStats `json:"stats"`
}

// JSONPath represents a file path in ripgrep JSON format
type JSONPath struct {
	Text string `json:"text"`
}

// JSONText represents text content in ripgrep JSON format
type JSONText struct {
	Text string `json:"text"`
}

// JSONSubmatch represents a submatch in ripgrep JSON format
type JSONSubmatch struct {
	Match JSONText `json:"match"`
	Start int      `json:"start"`
	End   int      `json:"end"`
}

// FormatFileBegin outputs a "begin" message for a file
func (f *JSONFormatter) FormatFileBegin(path string) error {
	msg := JSONMessage{
		Type: "begin",
		Data: JSONBeginData{
			Path: JSONPath{Text: path},
		},
	}
	return f.encoder.Encode(msg)
}

// FormatMatch formats a single match in ripgrep-compatible JSON format
func (f *JSONFormatter) FormatMatch(match Match) error {
	// Convert submatches to JSON format
	jsonSubmatches := make([]JSONSubmatch, len(match.Submatches))
	for i, submatch := range match.Submatches {
		jsonSubmatches[i] = JSONSubmatch{
			Match: JSONText{Text: submatch.Text},
			Start: submatch.Start,
			End:   submatch.End,
		}
	}

	// Prepare the line text - ripgrep includes the newline
	lineText := match.Line
	if !strings.HasSuffix(lineText, "\n") {
		lineText += "\n"
	}

	msg := JSONMessage{
		Type: "match",
		Data: JSONMatchData{
			Path:           JSONPath{Text: match.Path},
			Lines:          JSONText{Text: lineText},
			LineNumber:     match.LineNumber,
			AbsoluteOffset: match.AbsoluteOffset,
			Submatches:     jsonSubmatches,
		},
	}

	return f.encoder.Encode(msg)
}

// FormatFileEnd outputs an "end" message for a file
func (f *JSONFormatter) FormatFileEnd(result FileResult) error {
	msg := JSONMessage{
		Type: "end",
		Data: JSONEndData{
			Path:         JSONPath{Text: result.Path},
			BinaryOffset: result.BinaryOffset,
			Stats:        result.Stats,
		},
	}

	// Store stats for summary
	f.stats[result.Path] = &result.Stats

	return f.encoder.Encode(msg)
}

// FormatSummary outputs a "summary" message with overall statistics
func (f *JSONFormatter) FormatSummary(summary SearchSummary) error {
	msg := JSONMessage{
		Type: "summary",
		Data: JSONSummaryData{
			ElapsedTotal: summary.ElapsedTotal,
			Stats:        summary.Stats,
		},
	}

	return f.encoder.Encode(msg)
}

// Flush flushes any buffered output
func (f *JSONFormatter) Flush() error {
	if flusher, ok := f.writer.(interface{ Flush() error }); ok {
		return flusher.Flush()
	}
	return nil
}

// Close closes the formatter
func (f *JSONFormatter) Close() error {
	return f.Flush()
}

// JSONLinesFormatter implements JSONL (JSON Lines) output format
// This is useful for streaming and processing large result sets
type JSONLinesFormatter struct {
	writer  io.Writer
	config  FormatterConfig
	encoder *json.Encoder
}

// NewJSONLinesFormatter creates a new JSON Lines formatter
func NewJSONLinesFormatter(writer io.Writer, config FormatterConfig) *JSONLinesFormatter {
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)

	return &JSONLinesFormatter{
		writer:  writer,
		config:  config,
		encoder: encoder,
	}
}

// JSONLinesMatch represents a match in JSON Lines format with optional semantic data
type JSONLinesMatch struct {
	Path           string        `json:"path"`
	Language       string        `json:"language,omitempty"`
	LineNumber     int           `json:"line_number"`
	ColumnNumber   int           `json:"column_number,omitempty"`
	AbsoluteOffset int64         `json:"absolute_offset"`
	Line           string        `json:"line"`
	Matches        []JSONSubmatch `json:"matches"`

	// Semantic extensions (beyond ripgrep)
	Semantic *SemanticInfo `json:"semantic,omitempty"`
}

// FormatFileBegin is a no-op for JSON Lines format
func (f *JSONLinesFormatter) FormatFileBegin(path string) error {
	return nil
}

// FormatMatch formats a single match in JSON Lines format
func (f *JSONLinesFormatter) FormatMatch(match Match) error {
	// Convert submatches to JSON format
	jsonSubmatches := make([]JSONSubmatch, len(match.Submatches))
	for i, submatch := range match.Submatches {
		jsonSubmatches[i] = JSONSubmatch{
			Match: JSONText{Text: submatch.Text},
			Start: submatch.Start,
			End:   submatch.End,
		}
	}

	jsonMatch := JSONLinesMatch{
		Path:           match.Path,
		Language:       match.Language,
		LineNumber:     match.LineNumber,
		ColumnNumber:   match.ColumnNumber,
		AbsoluteOffset: match.AbsoluteOffset,
		Line:           match.Line,
		Matches:        jsonSubmatches,
		Semantic:       match.Semantic,
	}

	return f.encoder.Encode(jsonMatch)
}

// FormatFileEnd is a no-op for JSON Lines format
func (f *JSONLinesFormatter) FormatFileEnd(result FileResult) error {
	return nil
}

// FormatSummary outputs summary in JSON Lines format
func (f *JSONLinesFormatter) FormatSummary(summary SearchSummary) error {
	summaryData := map[string]interface{}{
		"type":          "summary",
		"elapsed_total": summary.ElapsedTotal,
		"stats":         summary.Stats,
	}

	if len(summary.Files) > 0 {
		summaryData["files"] = summary.Files
	}

	return f.encoder.Encode(summaryData)
}

// Flush flushes any buffered output
func (f *JSONLinesFormatter) Flush() error {
	if flusher, ok := f.writer.(interface{ Flush() error }); ok {
		return flusher.Flush()
	}
	return nil
}

// Close closes the formatter
func (f *JSONLinesFormatter) Close() error {
	return f.Flush()
}

// SemanticJSONFormatter extends JSONFormatter with semantic information
// This maintains full ripgrep compatibility while adding semantic extensions
type SemanticJSONFormatter struct {
	*JSONFormatter
}

// NewSemanticJSONFormatter creates a JSON formatter with semantic extensions
func NewSemanticJSONFormatter(writer io.Writer, config FormatterConfig) *SemanticJSONFormatter {
	return &SemanticJSONFormatter{
		JSONFormatter: NewJSONFormatter(writer, config),
	}
}

// JSONSemanticMatchData extends JSONMatchData with semantic information
type JSONSemanticMatchData struct {
	JSONMatchData
	Semantic *SemanticInfo `json:"semantic,omitempty"`
}

// FormatMatch formats a match with optional semantic information
func (f *SemanticJSONFormatter) FormatMatch(match Match) error {
	// If no semantic information or not configured to include it, use standard format
	if match.Semantic == nil || !f.config.IncludeSemantic {
		return f.JSONFormatter.FormatMatch(match)
	}

	// Convert submatches to JSON format
	jsonSubmatches := make([]JSONSubmatch, len(match.Submatches))
	for i, submatch := range match.Submatches {
		jsonSubmatches[i] = JSONSubmatch{
			Match: JSONText{Text: submatch.Text},
			Start: submatch.Start,
			End:   submatch.End,
		}
	}

	// Prepare the line text - ripgrep includes the newline
	lineText := match.Line
	if !strings.HasSuffix(lineText, "\n") {
		lineText += "\n"
	}

	msg := JSONMessage{
		Type: "match",
		Data: JSONSemanticMatchData{
			JSONMatchData: JSONMatchData{
				Path:           JSONPath{Text: match.Path},
				Lines:          JSONText{Text: lineText},
				LineNumber:     match.LineNumber,
				AbsoluteOffset: match.AbsoluteOffset,
				Submatches:     jsonSubmatches,
			},
			Semantic: match.Semantic,
		},
	}

	return f.encoder.Encode(msg)
}

// ValidateRipgrepCompatibility checks if JSON output matches ripgrep format exactly
func ValidateRipgrepCompatibility(output []byte) error {
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")

	for i, line := range lines {
		var msg JSONMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			return fmt.Errorf("line %d: invalid JSON: %v", i+1, err)
		}

		// Validate message types
		switch msg.Type {
		case "begin", "match", "end", "summary":
			// Valid types
		default:
			return fmt.Errorf("line %d: invalid message type: %s", i+1, msg.Type)
		}

		// Validate required fields for each message type
		switch msg.Type {
		case "begin":
			var data JSONBeginData
			if err := remarshalData(msg.Data, &data); err != nil {
				return fmt.Errorf("line %d: invalid begin data: %v", i+1, err)
			}
			if data.Path.Text == "" {
				return fmt.Errorf("line %d: begin message missing path", i+1)
			}

		case "match":
			var data JSONMatchData
			if err := remarshalData(msg.Data, &data); err != nil {
				return fmt.Errorf("line %d: invalid match data: %v", i+1, err)
			}
			if data.Path.Text == "" || data.LineNumber == 0 {
				return fmt.Errorf("line %d: match message missing required fields", i+1)
			}

		case "end":
			var data JSONEndData
			if err := remarshalData(msg.Data, &data); err != nil {
				return fmt.Errorf("line %d: invalid end data: %v", i+1, err)
			}
			if data.Path.Text == "" {
				return fmt.Errorf("line %d: end message missing path", i+1)
			}

		case "summary":
			var data JSONSummaryData
			if err := remarshalData(msg.Data, &data); err != nil {
				return fmt.Errorf("line %d: invalid summary data: %v", i+1, err)
			}
		}
	}

	return nil
}

// remarshalData converts interface{} data back to a specific type
func remarshalData(data interface{}, target interface{}) error {
	bytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return json.Unmarshal(bytes, target)
}
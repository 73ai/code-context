package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
)

// OutputFormatter handles formatting search results for display
type RealOutputFormatter struct {
	config *Config
	colors ColorScheme
}

// ColorScheme defines ANSI color codes
type ColorScheme struct {
	FileName    string
	LineNumber  string
	Match       string
	Context     string
	Separator   string
	Reset       string
}

// JSONOutput represents the JSON output format (ripgrep compatible)
type JSONOutput struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// JSONMatch represents a match in JSON format
type JSONMatch struct {
	Path struct {
		Text string `json:"text"`
	} `json:"path"`
	Lines struct {
		Text string `json:"text"`
	} `json:"lines"`
	LineNumber    int    `json:"line_number"`
	AbsoluteOffset int   `json:"absolute_offset"`
	Submatches    []struct {
		Match struct {
			Text string `json:"text"`
		} `json:"match"`
		Start int `json:"start"`
		End   int `json:"end"`
	} `json:"submatches"`
}

// JSONStats represents statistics in JSON format
type JSONStats struct {
	ElapsedTotal struct {
		Secs  int `json:"secs"`
		Nanos int `json:"nanos"`
	} `json:"elapsed_total"`
	Searches    int `json:"searches"`
	SearchesWithMatch int `json:"searches_with_match"`
	BytesSearched int64 `json:"bytes_searched"`
	BytesPrinted  int64 `json:"bytes_printed"`
	MatchedLines  int64 `json:"matched_lines"`
	Matches       int64 `json:"matches"`
}

// NewRealOutputFormatter creates a new output formatter
func NewRealOutputFormatter(config *Config) *RealOutputFormatter {
	formatter := &RealOutputFormatter{
		config: config,
		colors: ColorScheme{
			FileName:   "\033[1;35m", // Magenta bold
			LineNumber: "\033[1;32m", // Green bold
			Match:      "\033[1;31m", // Red bold
			Context:    "\033[0;37m", // Gray
			Separator:  "\033[0;36m", // Cyan
			Reset:      "\033[0m",    // Reset
		},
	}

	// Disable colors based on configuration
	if !formatter.shouldUseColor() {
		formatter.colors = ColorScheme{} // All empty strings
	}

	return formatter
}

// Output formats and writes search results
func (f *RealOutputFormatter) Output(writer io.Writer, results interface{}) error {
	searchResults, ok := results.(*SearchResults)
	if !ok {
		return fmt.Errorf("invalid results type: expected *SearchResults")
	}

	if f.config.JSON {
		return f.outputJSON(writer, searchResults)
	}

	return f.outputText(writer, searchResults)
}

func (f *RealOutputFormatter) shouldUseColor() bool {
	switch f.config.Color {
	case "always":
		return true
	case "never":
		return false
	case "auto":
		// Check if output is a terminal
		return isTerminal(os.Stdout)
	default:
		return false
	}
}

func (f *RealOutputFormatter) outputJSON(writer io.Writer, results *SearchResults) error {
	encoder := json.NewEncoder(writer)

	// Output begin event
	if err := encoder.Encode(JSONOutput{
		Type: "begin",
		Data: map[string]interface{}{
			"timestamp": results.Timestamp,
			"pattern":   results.Pattern,
			"semantic":  results.Semantic,
		},
	}); err != nil {
		return err
	}

	// Handle different output modes
	if f.config.Count {
		return f.outputJSONCount(encoder, results)
	}

	if f.config.FilesWithMatches {
		return f.outputJSONFiles(encoder, results)
	}

	// Regular match output
	for _, result := range results.Results {
		jsonMatch := f.convertToJSONMatch(result)
		if err := encoder.Encode(JSONOutput{
			Type: "match",
			Data: jsonMatch,
		}); err != nil {
			return err
		}
	}

	// Output summary statistics
	stats := JSONStats{
		Searches:          results.Stats.FilesSearched,
		SearchesWithMatch: len(results.Results),
		MatchedLines:      int64(len(results.Results)),
		Matches:          int64(results.Stats.Matches),
	}
	stats.ElapsedTotal.Secs = int(results.Stats.Duration.Seconds())
	stats.ElapsedTotal.Nanos = int(results.Stats.Duration.Nanoseconds() % 1e9)

	return encoder.Encode(JSONOutput{
		Type: "summary",
		Data: stats,
	})
}

func (f *RealOutputFormatter) outputJSONCount(encoder *json.Encoder, results *SearchResults) error {
	counts := make(map[string]int)
	for _, result := range results.Results {
		counts[result.File]++
	}

	for file, count := range counts {
		if err := encoder.Encode(JSONOutput{
			Type: "match",
			Data: map[string]interface{}{
				"path": map[string]string{"text": file},
				"count": count,
			},
		}); err != nil {
			return err
		}
	}

	return nil
}

func (f *RealOutputFormatter) outputJSONFiles(encoder *json.Encoder, results *SearchResults) error {
	files := make(map[string]bool)
	for _, result := range results.Results {
		if !files[result.File] {
			files[result.File] = true
			if err := encoder.Encode(JSONOutput{
				Type: "match",
				Data: map[string]interface{}{
					"path": map[string]string{"text": result.File},
				},
			}); err != nil {
				return err
			}
		}
	}

	return nil
}

func (f *RealOutputFormatter) convertToJSONMatch(result SearchResult) JSONMatch {
	match := JSONMatch{
		LineNumber: result.LineNumber,
	}

	match.Path.Text = result.File
	match.Lines.Text = result.Match

	// Add submatch information
	if result.Column > 0 {
		submatch := struct {
			Match struct {
				Text string `json:"text"`
			} `json:"match"`
			Start int `json:"start"`
			End   int `json:"end"`
		}{}
		submatch.Match.Text = result.Match
		submatch.Start = result.Column - 1
		submatch.End = result.Column - 1 + len(result.Match)
		match.Submatches = append(match.Submatches, submatch)
	}

	return match
}

func (f *RealOutputFormatter) outputText(writer io.Writer, results *SearchResults) error {
	if f.config.Count {
		return f.outputTextCount(writer, results)
	}

	if f.config.FilesWithMatches {
		return f.outputTextFiles(writer, results)
	}

	// Group results by file
	fileResults := f.groupResultsByFile(results.Results)

	// Sort files for consistent output
	var files []string
	for file := range fileResults {
		files = append(files, file)
	}
	sort.Strings(files)

	// Output results for each file
	for i, file := range files {
		if i > 0 && !f.config.NoHeading {
			// Add separator between files
			fmt.Fprintln(writer)
		}

		results := fileResults[file]
		if err := f.outputFileResults(writer, file, results); err != nil {
			return err
		}
	}

	// Output summary if not quiet
	if !f.config.Quiet {
		return f.outputTextSummary(writer, results)
	}

	return nil
}

func (f *RealOutputFormatter) outputTextCount(writer io.Writer, results *SearchResults) error {
	counts := make(map[string]int)
	for _, result := range results.Results {
		counts[result.File]++
	}

	// Sort files for consistent output
	var files []string
	for file := range counts {
		files = append(files, file)
	}
	sort.Strings(files)

	for _, file := range files {
		count := counts[file]
		if f.config.WithFilename || len(files) > 1 {
			fmt.Fprintf(writer, "%s%s%s%s:%d\n",
				f.colors.FileName, file, f.colors.Reset,
				f.colors.Separator, count)
		} else {
			fmt.Fprintf(writer, "%d\n", count)
		}
	}

	return nil
}

func (f *RealOutputFormatter) outputTextFiles(writer io.Writer, results *SearchResults) error {
	files := make(map[string]bool)
	var fileList []string

	for _, result := range results.Results {
		if !files[result.File] {
			files[result.File] = true
			fileList = append(fileList, result.File)
		}
	}

	sort.Strings(fileList)
	for _, file := range fileList {
		fmt.Fprintf(writer, "%s%s%s\n",
			f.colors.FileName, file, f.colors.Reset)
	}

	return nil
}

func (f *RealOutputFormatter) groupResultsByFile(results []SearchResult) map[string][]SearchResult {
	fileResults := make(map[string][]SearchResult)
	for _, result := range results {
		fileResults[result.File] = append(fileResults[result.File], result)
	}

	// Sort results within each file by line number
	for file := range fileResults {
		sort.Slice(fileResults[file], func(i, j int) bool {
			return fileResults[file][i].LineNumber < fileResults[file][j].LineNumber
		})
	}

	return fileResults
}

func (f *RealOutputFormatter) outputFileResults(writer io.Writer, file string, results []SearchResult) error {
	// Print file heading unless disabled
	if !f.config.NoHeading && (f.config.WithFilename || len(results) > 1) {
		fmt.Fprintf(writer, "%s%s%s\n",
			f.colors.FileName, file, f.colors.Reset)
	}

	for _, result := range results {
		if err := f.outputSingleResult(writer, result); err != nil {
			return err
		}
	}

	return nil
}

func (f *RealOutputFormatter) outputSingleResult(writer io.Writer, result SearchResult) error {
	var parts []string

	// Add filename if requested
	if f.config.WithFilename && f.config.NoHeading {
		parts = append(parts, f.colors.FileName+result.File+f.colors.Reset)
	}

	// Add line number if requested
	if f.config.LineNumber {
		parts = append(parts, f.colors.LineNumber+strconv.Itoa(result.LineNumber)+f.colors.Reset)
	}

	// Join parts with separator
	prefix := strings.Join(parts, f.colors.Separator+":"+f.colors.Reset)
	if len(parts) > 0 {
		prefix += f.colors.Separator + ":" + f.colors.Reset
	}

	// Output context before if available
	if context, ok := result.Context["before"]; ok {
		beforeLines := strings.Split(context, "\n")
		for i, line := range beforeLines {
			contextLineNum := result.LineNumber - len(beforeLines) + i
			f.outputContextLine(writer, result.File, contextLineNum, line, "-")
		}
	}

	// Output the main match line
	matchLine := result.Match
	if f.config.OnlyMatching {
		matchLine = f.highlightMatch(matchLine, result.Match)
	} else {
		matchLine = f.highlightMatches(matchLine)
	}

	fmt.Fprintf(writer, "%s%s\n", prefix, matchLine)

	// Output context after if available
	if context, ok := result.Context["after"]; ok {
		afterLines := strings.Split(context, "\n")
		for i, line := range afterLines {
			contextLineNum := result.LineNumber + 1 + i
			f.outputContextLine(writer, result.File, contextLineNum, line, "+")
		}
	}

	return nil
}

func (f *RealOutputFormatter) outputContextLine(writer io.Writer, file string, lineNum int, line, indicator string) {
	var parts []string

	// Add filename if requested
	if f.config.WithFilename && f.config.NoHeading {
		parts = append(parts, f.colors.FileName+file+f.colors.Reset)
	}

	// Add line number if requested
	if f.config.LineNumber {
		parts = append(parts, f.colors.Context+strconv.Itoa(lineNum)+f.colors.Reset)
	}

	prefix := strings.Join(parts, f.colors.Separator+indicator+f.colors.Reset)
	if len(parts) > 0 {
		prefix += f.colors.Separator + indicator + f.colors.Reset
	}

	fmt.Fprintf(writer, "%s%s%s%s\n",
		prefix, f.colors.Context, line, f.colors.Reset)
}

func (f *RealOutputFormatter) highlightMatch(line, match string) string {
	if match == "" {
		return line
	}
	return f.colors.Match + match + f.colors.Reset
}

func (f *RealOutputFormatter) highlightMatches(line string) string {
	// This is a simplified version - in a real implementation,
	// we would need to track match positions from the regex engine
	// For now, just return the line as-is with basic coloring
	return line
}

func (f *RealOutputFormatter) outputTextSummary(writer io.Writer, results *SearchResults) error {
	if len(results.Results) == 0 && !f.config.Quiet {
		fmt.Fprintf(writer, "No matches found\n")
	}
	return nil
}

// Platform-specific terminal detection
// This would be implemented differently for different platforms
func isTerminal(file *os.File) bool {
	// Simplified implementation - in real code, use syscalls
	// or a library like github.com/mattn/go-isatty
	return file == os.Stdout && os.Getenv("TERM") != ""
}

// Update the newOutputFormatter function in root.go
func newOutputFormatter(config *Config) OutputFormatter {
	return NewRealOutputFormatter(config)
}
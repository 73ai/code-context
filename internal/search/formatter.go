package search

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Formatter handles output formatting for search results
type Formatter struct {
	opts   *SearchOptions
	writer io.Writer
}

func NewFormatter(opts *SearchOptions, writer io.Writer) *Formatter {
	return &Formatter{
		opts:   opts,
		writer: writer,
	}
}

func (f *Formatter) FormatResult(result *SearchResult) error {
	if f.opts.JSON {
		return f.formatJSON(result)
	}
	return f.formatText(result)
}

// FormatStats formats and outputs search statistics
func (f *Formatter) FormatStats(stats *SearchStats) error {
	if f.opts.JSON {
		return f.formatStatsJSON(stats)
	}
	return f.formatStatsText(stats)
}

// formatJSON formats result as JSON (ripgrep compatible)
func (f *Formatter) formatJSON(result *SearchResult) error {
	data := map[string]interface{}{
		"type": "match",
		"data": map[string]interface{}{
			"path": map[string]interface{}{
				"text": result.FilePath,
			},
			"lines": map[string]interface{}{
				"text": result.Line,
			},
			"line_number":   result.LineNumber,
			"absolute_offset": 0, // TODO: Calculate actual offset
			"submatches": []map[string]interface{}{
				{
					"match": map[string]interface{}{
						"text": result.Match,
					},
					"start": result.ColumnStart - 1, // Zero-based for ripgrep compatibility
					"end":   result.ColumnEnd - 1,
				},
			},
		},
	}

	// Add semantic metadata if available
	if result.SymbolName != "" {
		data["data"].(map[string]interface{})["symbol_name"] = result.SymbolName
		data["data"].(map[string]interface{})["symbol_type"] = result.SymbolType
		data["data"].(map[string]interface{})["symbol_kind"] = result.SymbolKind
	}

	if result.Scope != "" {
		data["data"].(map[string]interface{})["scope"] = result.Scope
	}

	if len(result.Metadata) > 0 {
		data["data"].(map[string]interface{})["metadata"] = result.Metadata
	}

	encoder := json.NewEncoder(f.writer)
	return encoder.Encode(data)
}

// formatText formats result as plain text
func (f *Formatter) formatText(result *SearchResult) error {
	var output strings.Builder

	// File path
	if f.opts.WithFilename || len(f.opts.SearchPaths) > 1 {
		if f.opts.NoHeading {
			output.WriteString(result.FilePath)
			output.WriteString(":")
		} else {
			fmt.Fprintf(&output, "\n%s\n", result.FilePath)
		}
	}

	// Line number
	if f.opts.LineNumbers {
		fmt.Fprintf(&output, "%d:", result.LineNumber)
	}

	// Column number (if available)
	if result.ColumnStart > 0 {
		fmt.Fprintf(&output, "%d:", result.ColumnStart)
	}

	// Content
	if result.Line != "" {
		output.WriteString(result.Line)
	} else if result.Match != "" {
		output.WriteString(result.Match)
	}

	// Semantic information
	if result.SymbolName != "" {
		fmt.Fprintf(&output, " [%s: %s]", result.SymbolKind, result.SymbolName)
	}

	output.WriteString("\n")

	// Context lines
	if len(result.Context) > 0 {
		for i, contextLine := range result.Context {
			lineNum := result.LineNumber + i - len(result.Context)/2
			if f.opts.LineNumbers {
				fmt.Fprintf(&output, "%d-", lineNum)
			}
			fmt.Fprintf(&output, "%s\n", contextLine)
		}
	}

	_, err := f.writer.Write([]byte(output.String()))
	return err
}

// formatStatsJSON formats statistics as JSON
func (f *Formatter) formatStatsJSON(stats *SearchStats) error {
	data := map[string]interface{}{
		"type": "stats",
		"data": map[string]interface{}{
			"elapsed":          stats.SearchDuration.Seconds(),
			"files_searched":   stats.FilesSearched,
			"files_skipped":    stats.FilesSkipped,
			"total_matches":    stats.TotalMatches,
			"total_files":      stats.TotalFiles,
			"bytes_searched":   stats.BytesSearched,
			"peak_memory_usage": stats.PeakMemoryUsage,
		},
	}

	if stats.IndexDuration > 0 {
		data["data"].(map[string]interface{})["index_duration"] = stats.IndexDuration.Seconds()
	}

	encoder := json.NewEncoder(f.writer)
	return encoder.Encode(data)
}

// formatStatsText formats statistics as plain text
func (f *Formatter) formatStatsText(stats *SearchStats) error {
	var output strings.Builder

	fmt.Fprintf(&output, "\nSearch Statistics:\n")
	fmt.Fprintf(&output, "  Files searched: %d\n", stats.FilesSearched)
	fmt.Fprintf(&output, "  Total matches: %d\n", stats.TotalMatches)
	fmt.Fprintf(&output, "  Search duration: %v\n", stats.SearchDuration)

	if stats.IndexDuration > 0 {
		fmt.Fprintf(&output, "  Index duration: %v\n", stats.IndexDuration)
	}

	fmt.Fprintf(&output, "  Bytes searched: %d\n", stats.BytesSearched)
	fmt.Fprintf(&output, "  Peak memory usage: %d bytes\n", stats.PeakMemoryUsage)

	_, err := f.writer.Write([]byte(output.String()))
	return err
}

// Close finalizes the formatter (placeholder for future needs)
func (f *Formatter) Close() error {
	return nil
}
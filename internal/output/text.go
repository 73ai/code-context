package output

import (
	"io"
	"strconv"
	"strings"
)

// ANSI color codes for output highlighting
const (
	// Reset
	Reset = "\033[0m"

	// Colors
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
	White   = "\033[37m"

	// Bright colors
	BrightRed     = "\033[91m"
	BrightGreen   = "\033[92m"
	BrightYellow  = "\033[93m"
	BrightBlue    = "\033[94m"
	BrightMagenta = "\033[95m"
	BrightCyan    = "\033[96m"

	// Styles
	Bold      = "\033[1m"
	Underline = "\033[4m"
)

// TextFormatter implements ripgrep-compatible text output
type TextFormatter struct {
	writer     io.Writer
	config     FormatterConfig
	totalCount int
}

// NewTextFormatter creates a new text formatter
func NewTextFormatter(writer io.Writer, config FormatterConfig) *TextFormatter {
	return &TextFormatter{
		writer: writer,
		config: config,
	}
}

// FormatMatch formats a single match in ripgrep-compatible text format
func (f *TextFormatter) FormatMatch(match Match) error {
	switch f.config.Mode {
	case ModeFilesOnly:
		// Files-only mode is handled by FilesFormatter
		return nil
	case ModeCount:
		// Count mode is handled by CountFormatter
		f.totalCount++
		return nil
	case ModeOnlyMatching:
		return f.formatOnlyMatching(match)
	default:
		return f.formatDefault(match)
	}
}

// formatDefault formats a match in the default ripgrep style
func (f *TextFormatter) formatDefault(match Match) error {
	// Format context before
	for _, ctx := range match.BeforeContext {
		if err := f.writeContextLine(match.Path, ctx, "-"); err != nil {
			return err
		}
	}

	// Format the main match line
	if err := f.writeMatchLine(match); err != nil {
		return err
	}

	// Format context after
	for _, ctx := range match.AfterContext {
		if err := f.writeContextLine(match.Path, ctx, "-"); err != nil {
			return err
		}
	}

	return nil
}

// formatOnlyMatching formats only the matching portions
func (f *TextFormatter) formatOnlyMatching(match Match) error {
	for _, submatch := range match.Submatches {
		line := f.formatFilePath(match.Path)
		if f.config.ShowLineNumbers {
			line += ":" + f.colorize(strconv.Itoa(match.LineNumber), Green)
		}
		line += ":" + f.highlightMatch(submatch.Text) + "\n"

		if _, err := f.writer.Write([]byte(line)); err != nil {
			return err
		}
	}
	return nil
}

// writeMatchLine writes a single match line with highlighting
func (f *TextFormatter) writeMatchLine(match Match) error {
	var line strings.Builder

	// Add file path if configured
	if f.config.ShowFilenames {
		line.WriteString(f.formatFilePath(match.Path))
		line.WriteString(":")
	}

	// Add line number if configured
	if f.config.ShowLineNumbers {
		line.WriteString(f.colorize(strconv.Itoa(match.LineNumber), Green))
		line.WriteString(":")
	}

	// Add the line content with highlighting
	highlightedLine := f.highlightLine(match.Line, match.Submatches)
	line.WriteString(highlightedLine)

	line.WriteString("\n")

	_, err := f.writer.Write([]byte(line.String()))
	return err
}

// writeContextLine writes a context line
func (f *TextFormatter) writeContextLine(path string, ctx ContextLine, separator string) error {
	var line strings.Builder

	// Add file path if configured
	if f.config.ShowFilenames {
		line.WriteString(f.formatFilePath(path))
		line.WriteString(separator)
	}

	// Add line number if configured
	if f.config.ShowLineNumbers {
		line.WriteString(f.colorize(strconv.Itoa(ctx.LineNumber), Green))
		line.WriteString(separator)
	}

	line.WriteString(ctx.Text)
	line.WriteString("\n")

	_, err := f.writer.Write([]byte(line.String()))
	return err
}

// formatFilePath formats a file path with appropriate colors
func (f *TextFormatter) formatFilePath(path string) string {
	if f.config.ShowColors {
		return f.colorize(path, Magenta)
	}
	return path
}

// highlightLine highlights matches within a line
func (f *TextFormatter) highlightLine(line string, submatches []Submatch) string {
	if !f.config.ShowColors || len(submatches) == 0 {
		return line
	}

	// Sort submatches by start position to ensure correct highlighting
	sortedMatches := make([]Submatch, len(submatches))
	copy(sortedMatches, submatches)

	// Simple bubble sort (fine for small number of matches per line)
	for i := 0; i < len(sortedMatches); i++ {
		for j := i + 1; j < len(sortedMatches); j++ {
			if sortedMatches[i].Start > sortedMatches[j].Start {
				sortedMatches[i], sortedMatches[j] = sortedMatches[j], sortedMatches[i]
			}
		}
	}

	var result strings.Builder
	lastEnd := 0

	for _, match := range sortedMatches {
		// Add text before the match
		if match.Start > lastEnd {
			result.WriteString(line[lastEnd:match.Start])
		}

		// Add highlighted match
		if match.End <= len(line) {
			result.WriteString(f.highlightMatch(line[match.Start:match.End]))
			lastEnd = match.End
		}
	}

	// Add remaining text after the last match
	if lastEnd < len(line) {
		result.WriteString(line[lastEnd:])
	}

	return result.String()
}

// highlightMatch highlights a match with the configured color
func (f *TextFormatter) highlightMatch(text string) string {
	if !f.config.ShowColors {
		return text
	}
	return f.colorize(text, BrightRed)
}

// colorize applies ANSI color codes to text
func (f *TextFormatter) colorize(text, color string) string {
	if !f.config.ShowColors {
		return text
	}
	return color + text + Reset
}

// FormatFileBegin formats the beginning of a file (no-op for text format)
func (f *TextFormatter) FormatFileBegin(path string) error {
	// Text format doesn't need file begin markers
	return nil
}

// FormatFileEnd formats the end of a file (no-op for text format)
func (f *TextFormatter) FormatFileEnd(result FileResult) error {
	// Text format doesn't need file end markers
	return nil
}

// FormatSummary formats the search summary (no-op for text format)
func (f *TextFormatter) FormatSummary(summary SearchSummary) error {
	// Text format doesn't output summaries by default
	return nil
}

// Flush flushes any buffered output
func (f *TextFormatter) Flush() error {
	if flusher, ok := f.writer.(interface{ Flush() error }); ok {
		return flusher.Flush()
	}
	return nil
}

// Close closes the formatter
func (f *TextFormatter) Close() error {
	return f.Flush()
}

// CountFormatter implements count-only output
type CountFormatter struct {
	writer    io.Writer
	config    FormatterConfig
	counts    map[string]int
	fileCount int
}

// NewCountFormatter creates a new count formatter
func NewCountFormatter(writer io.Writer, config FormatterConfig) *CountFormatter {
	return &CountFormatter{
		writer:    writer,
		config:    config,
		counts:    make(map[string]int),
		fileCount: 0,
	}
}

// FormatMatch counts matches per file
func (f *CountFormatter) FormatMatch(match Match) error {
	f.counts[match.Path] += len(match.Submatches)
	return nil
}

// FormatFileBegin does nothing for count format
func (f *CountFormatter) FormatFileBegin(path string) error {
	return nil
}

// FormatFileEnd outputs the count for a file
func (f *CountFormatter) FormatFileEnd(result FileResult) error {
	count := f.counts[result.Path]
	if count == 0 {
		return nil
	}

	var line strings.Builder

	// Add file path if configured and there are multiple files (ripgrep behavior)
	// Only show filename if there are multiple files OR ShowFilenames is explicitly true AND TotalFiles > 1
	showFilename := f.config.ShowFilenames && f.config.TotalFiles > 1
	if showFilename {
		if f.config.ShowColors {
			line.WriteString(f.colorize(result.Path, Magenta))
		} else {
			line.WriteString(result.Path)
		}
		line.WriteString(":")
	}

	line.WriteString(strconv.Itoa(count))
	line.WriteString("\n")

	_, err := f.writer.Write([]byte(line.String()))
	return err
}

// colorize applies ANSI color codes to text
func (f *CountFormatter) colorize(text, color string) string {
	if !f.config.ShowColors {
		return text
	}
	return color + text + Reset
}

// FormatSummary does nothing for count format
func (f *CountFormatter) FormatSummary(summary SearchSummary) error {
	return nil
}

// Flush flushes any buffered output
func (f *CountFormatter) Flush() error {
	if flusher, ok := f.writer.(interface{ Flush() error }); ok {
		return flusher.Flush()
	}
	return nil
}

// Close closes the formatter
func (f *CountFormatter) Close() error {
	return f.Flush()
}

// FilesFormatter implements files-only output
type FilesFormatter struct {
	writer     io.Writer
	config     FormatterConfig
	seenFiles  map[string]bool
}

// NewFilesFormatter creates a new files formatter
func NewFilesFormatter(writer io.Writer, config FormatterConfig) *FilesFormatter {
	return &FilesFormatter{
		writer:    writer,
		config:    config,
		seenFiles: make(map[string]bool),
	}
}

// FormatMatch tracks files with matches
func (f *FilesFormatter) FormatMatch(match Match) error {
	if !f.seenFiles[match.Path] {
		f.seenFiles[match.Path] = true
		line := match.Path + "\n"
		if f.config.ShowColors {
			line = f.colorize(match.Path, Magenta) + "\n"
		}
		_, err := f.writer.Write([]byte(line))
		return err
	}
	return nil
}

// FormatFileBegin does nothing for files format
func (f *FilesFormatter) FormatFileBegin(path string) error {
	return nil
}

// FormatFileEnd does nothing for files format
func (f *FilesFormatter) FormatFileEnd(result FileResult) error {
	return nil
}

// colorize applies ANSI color codes to text
func (f *FilesFormatter) colorize(text, color string) string {
	if !f.config.ShowColors {
		return text
	}
	return color + text + Reset
}

// FormatSummary does nothing for files format
func (f *FilesFormatter) FormatSummary(summary SearchSummary) error {
	return nil
}

// Flush flushes any buffered output
func (f *FilesFormatter) Flush() error {
	if flusher, ok := f.writer.(interface{ Flush() error }); ok {
		return flusher.Flush()
	}
	return nil
}

// Close closes the formatter
func (f *FilesFormatter) Close() error {
	return f.Flush()
}
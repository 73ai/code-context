// Package walker provides file filtering capabilities
package walker

import (
	"bufio"
	"bytes"
	"io/fs"
	"mime"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// Language represents a programming language or file type
type Language struct {
	Name       string   // Language name (e.g., "Go", "JavaScript")
	Extensions []string // File extensions (e.g., [".go", ".mod"])
	Patterns   []string // Filename patterns (e.g., ["Makefile", "*.mk"])
	MimeTypes  []string // MIME types
	Shebangs   []string // Shebang patterns for script detection
}

// FileType represents the detected type of a file
type FileType struct {
	Language   *Language // Detected language
	IsBinary   bool      // Whether the file is binary
	IsText     bool      // Whether the file is text
	MimeType   string    // Detected MIME type
	Confidence float64   // Detection confidence (0.0 to 1.0)
}

// Filters provides file filtering and language detection
type Filters struct {
	languages        map[string]*Language // Language definitions by name
	extensionMap     map[string]*Language // Extension to language mapping
	patternMap       map[string]*Language // Pattern to language mapping
	includedTypes    map[string]bool      // Explicitly included file types
	excludedTypes    map[string]bool      // Explicitly excluded file types
	includedExts     map[string]bool      // Explicitly included extensions
	excludedExts     map[string]bool      // Explicitly excluded extensions
	maxSize          int64                // Maximum file size to consider
	minSize          int64                // Minimum file size to consider
	binaryDetection  bool                 // Whether to perform binary detection
	textDetection    bool                 // Whether to perform text type detection
	allowHidden      bool                 // Whether to allow hidden files
	customPatterns   []*regexp.Regexp     // Custom filename patterns
	mu               sync.RWMutex         // Protects filter configuration
}

// NewFilters creates a new file filter with default language definitions
func NewFilters() *Filters {
	f := &Filters{
		languages:       make(map[string]*Language),
		extensionMap:    make(map[string]*Language),
		patternMap:      make(map[string]*Language),
		includedTypes:   make(map[string]bool),
		excludedTypes:   make(map[string]bool),
		includedExts:    make(map[string]bool),
		excludedExts:    make(map[string]bool),
		maxSize:         50 * 1024 * 1024, // 50MB default max size
		minSize:         0,
		binaryDetection: true,
		textDetection:   true,
		allowHidden:     false,
	}

	f.loadDefaultLanguages()
	return f
}

// loadDefaultLanguages loads common programming language definitions
func (f *Filters) loadDefaultLanguages() {
	languages := []*Language{
		{
			Name:       "Go",
			Extensions: []string{".go", ".mod", ".sum"},
			Patterns:   []string{"go.mod", "go.sum", "go.work", "go.work.sum"},
			MimeTypes:  []string{"text/x-go"},
			Shebangs:   []string{"go run"},
		},
		{
			Name:       "JavaScript",
			Extensions: []string{".js", ".mjs", ".jsx", ".es6", ".es"},
			Patterns:   []string{".eslintrc.js", ".babelrc.js"},
			MimeTypes:  []string{"application/javascript", "text/javascript"},
			Shebangs:   []string{"node", "nodejs"},
		},
		{
			Name:       "TypeScript",
			Extensions: []string{".ts", ".tsx", ".d.ts"},
			Patterns:   []string{"tsconfig.json", ".tslintrc"},
			MimeTypes:  []string{"application/typescript"},
		},
		{
			Name:       "Python",
			Extensions: []string{".py", ".pyw", ".pyi", ".pyx", ".pyd"},
			Patterns:   []string{"Pipfile", "pyproject.toml", "setup.py", "requirements.txt"},
			MimeTypes:  []string{"text/x-python", "application/x-python"},
			Shebangs:   []string{"python", "python2", "python3"},
		},
		{
			Name:       "Java",
			Extensions: []string{".java", ".class", ".jar", ".gradle"},
			Patterns:   []string{"build.gradle", "pom.xml", "gradle.properties"},
			MimeTypes:  []string{"text/x-java-source"},
		},
		{
			Name:       "C",
			Extensions: []string{".c", ".h"},
			MimeTypes:  []string{"text/x-c", "text/x-csrc"},
		},
		{
			Name:       "C++",
			Extensions: []string{".cpp", ".cxx", ".cc", ".c++", ".hpp", ".hxx", ".hh", ".h++"},
			MimeTypes:  []string{"text/x-c++", "text/x-c++src"},
		},
		{
			Name:       "Rust",
			Extensions: []string{".rs", ".rlib"},
			Patterns:   []string{"Cargo.toml", "Cargo.lock"},
			MimeTypes:  []string{"text/rust"},
		},
		{
			Name:       "C#",
			Extensions: []string{".cs", ".csx", ".csproj", ".sln"},
			MimeTypes:  []string{"text/x-csharp"},
		},
		{
			Name:       "Ruby",
			Extensions: []string{".rb", ".rbw", ".rake", ".gemspec"},
			Patterns:   []string{"Rakefile", "Gemfile", "Gemfile.lock"},
			MimeTypes:  []string{"text/x-ruby"},
			Shebangs:   []string{"ruby"},
		},
		{
			Name:       "PHP",
			Extensions: []string{".php", ".php3", ".php4", ".php5", ".phtml"},
			MimeTypes:  []string{"text/x-php", "application/x-php"},
			Shebangs:   []string{"php"},
		},
		{
			Name:       "Shell",
			Extensions: []string{".sh", ".bash", ".zsh", ".fish", ".csh", ".tcsh", ".ksh"},
			MimeTypes:  []string{"text/x-shellscript", "application/x-sh"},
			Shebangs:   []string{"sh", "bash", "zsh", "fish", "csh", "tcsh", "ksh"},
		},
		{
			Name:       "HTML",
			Extensions: []string{".html", ".htm", ".xhtml"},
			MimeTypes:  []string{"text/html", "application/xhtml+xml"},
		},
		{
			Name:       "CSS",
			Extensions: []string{".css", ".scss", ".sass", ".less"},
			MimeTypes:  []string{"text/css"},
		},
		{
			Name:       "JSON",
			Extensions: []string{".json", ".jsonc", ".json5"},
			Patterns:   []string{".eslintrc", ".babelrc", "tsconfig.json", "package.json"},
			MimeTypes:  []string{"application/json", "text/json"},
		},
		{
			Name:       "YAML",
			Extensions: []string{".yaml", ".yml"},
			Patterns:   []string{".github/workflows/*.yml", ".github/workflows/*.yaml"},
			MimeTypes:  []string{"text/yaml", "application/x-yaml"},
		},
		{
			Name:       "XML",
			Extensions: []string{".xml", ".xsl", ".xslt", ".xsd"},
			Patterns:   []string{"pom.xml", "web.xml"},
			MimeTypes:  []string{"text/xml", "application/xml"},
		},
		{
			Name:       "Markdown",
			Extensions: []string{".md", ".markdown", ".mdown", ".mkd"},
			Patterns:   []string{"README.md", "CHANGELOG.md"},
			MimeTypes:  []string{"text/markdown", "text/x-markdown"},
		},
		{
			Name:       "SQL",
			Extensions: []string{".sql"},
			MimeTypes:  []string{"text/x-sql"},
		},
		{
			Name:       "Docker",
			Extensions: []string{".dockerfile"},
			Patterns:   []string{"Dockerfile", "Dockerfile.*", ".dockerignore"},
			MimeTypes:  []string{"text/x-dockerfile"},
		},
		{
			Name:       "Makefile",
			Extensions: []string{".mk", ".mak"},
			Patterns:   []string{"Makefile", "makefile", "GNUmakefile", "*.mk"},
			MimeTypes:  []string{"text/x-makefile"},
		},
	}

	for _, lang := range languages {
		f.AddLanguage(lang)
	}
}

// AddLanguage adds a language definition to the filter
func (f *Filters) AddLanguage(lang *Language) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.languages[strings.ToLower(lang.Name)] = lang

	// Map extensions
	for _, ext := range lang.Extensions {
		f.extensionMap[strings.ToLower(ext)] = lang
	}

	// Map patterns
	for _, pattern := range lang.Patterns {
		f.patternMap[strings.ToLower(pattern)] = lang
	}
}

// ShouldInclude determines if a file should be included based on current filters
func (f *Filters) ShouldInclude(path string, info fs.FileInfo) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()

	// Check size limits
	if f.maxSize > 0 && info.Size() > f.maxSize {
		return false
	}
	if f.minSize > 0 && info.Size() < f.minSize {
		return false
	}

	// Check hidden files
	if !f.allowHidden && isHiddenFile(filepath.Base(path)) {
		return false
	}

	// Check explicit extension filters
	ext := strings.ToLower(filepath.Ext(path))
	if len(f.excludedExts) > 0 && f.excludedExts[ext] {
		return false
	}
	if len(f.includedExts) > 0 && !f.includedExts[ext] {
		return false
	}

	// Detect file type
	fileType := f.DetectType(path, info)

	// Check type filters
	if fileType.Language != nil {
		langName := strings.ToLower(fileType.Language.Name)
		if len(f.excludedTypes) > 0 && f.excludedTypes[langName] {
			return false
		}
		if len(f.includedTypes) > 0 && !f.includedTypes[langName] {
			return false
		}
	}

	// Check custom patterns
	for _, pattern := range f.customPatterns {
		if pattern.MatchString(filepath.Base(path)) {
			return true
		}
	}

	return true
}

// DetectType analyzes a file and determines its type and language
func (f *Filters) DetectType(path string, info fs.FileInfo) *FileType {
	result := &FileType{
		IsText: true,
		IsBinary: false,
		Confidence: 0.0,
	}

	// First, try extension-based detection
	if lang := f.detectByExtension(path); lang != nil {
		result.Language = lang
		result.Confidence = 0.8
	}

	// Try pattern-based detection
	if result.Language == nil {
		if lang := f.detectByPattern(path); lang != nil {
			result.Language = lang
			result.Confidence = 0.7
		}
	}

	// Try MIME type detection
	if mimeType := mime.TypeByExtension(filepath.Ext(path)); mimeType != "" {
		result.MimeType = mimeType
		if result.Language == nil {
			if lang := f.detectByMimeType(mimeType); lang != nil {
				result.Language = lang
				result.Confidence = 0.6
			}
		}
	}

	// Perform binary detection if enabled
	if f.binaryDetection {
		if isBinary := f.isBinaryFile(path); isBinary {
			result.IsBinary = true
			result.IsText = false
		}
	}

	// Try shebang detection for scripts
	if result.Language == nil && result.IsText {
		if lang := f.detectByShebang(path); lang != nil {
			result.Language = lang
			result.Confidence = 0.9
		}
	}

	return result
}

// detectByExtension attempts to detect language by file extension
func (f *Filters) detectByExtension(path string) *Language {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == "" {
		return nil
	}
	return f.extensionMap[ext]
}

// detectByPattern attempts to detect language by filename patterns
func (f *Filters) detectByPattern(path string) *Language {
	filename := strings.ToLower(filepath.Base(path))

	// Check exact matches first
	if lang, ok := f.patternMap[filename]; ok {
		return lang
	}

	// Check glob patterns
	for pattern, lang := range f.patternMap {
		if matched, _ := filepath.Match(pattern, filename); matched {
			return lang
		}
	}

	return nil
}

// detectByMimeType attempts to detect language by MIME type
func (f *Filters) detectByMimeType(mimeType string) *Language {
	for _, lang := range f.languages {
		for _, mt := range lang.MimeTypes {
			if strings.EqualFold(mt, mimeType) {
				return lang
			}
		}
	}
	return nil
}

// detectByShebang attempts to detect language by shebang line
func (f *Filters) detectByShebang(path string) *Language {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()

	// Read first line
	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		return nil
	}

	firstLine := strings.TrimSpace(scanner.Text())
	if !strings.HasPrefix(firstLine, "#!") {
		return nil
	}

	shebang := strings.ToLower(firstLine[2:])

	// Check against language shebangs
	for _, lang := range f.languages {
		for _, sb := range lang.Shebangs {
			if strings.Contains(shebang, strings.ToLower(sb)) {
				return lang
			}
		}
	}

	return nil
}

// isBinaryFile determines if a file is binary by examining its content
func (f *Filters) isBinaryFile(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()

	// Read first 512 bytes for detection
	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && n == 0 {
		return false
	}

	return isBinaryContent(buffer[:n])
}

// isBinaryContent checks if the given content is binary
func isBinaryContent(content []byte) bool {
	if len(content) == 0 {
		return false
	}

	// Check for null bytes (strong indicator of binary content)
	if bytes.Contains(content, []byte{0}) {
		return true
	}

	// Count non-printable characters
	nonPrintable := 0
	for _, b := range content {
		if b < 32 && b != 9 && b != 10 && b != 13 {
			nonPrintable++
		}
	}

	// If more than 30% are non-printable, consider it binary
	if len(content) > 0 {
		ratio := float64(nonPrintable) / float64(len(content))
		return ratio > 0.30
	}

	return false
}

// Configuration methods

// IncludeType adds a language type to be included
func (f *Filters) IncludeType(langName string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.includedTypes[strings.ToLower(langName)] = true
}

// ExcludeType adds a language type to be excluded
func (f *Filters) ExcludeType(langName string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.excludedTypes[strings.ToLower(langName)] = true
}

// IncludeExtension adds a file extension to be included
func (f *Filters) IncludeExtension(ext string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	f.includedExts[strings.ToLower(ext)] = true
}

// ExcludeExtension adds a file extension to be excluded
func (f *Filters) ExcludeExtension(ext string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	f.excludedExts[strings.ToLower(ext)] = true
}

// SetSizeRange sets the minimum and maximum file sizes to consider
func (f *Filters) SetSizeRange(minSize, maxSize int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.minSize = minSize
	f.maxSize = maxSize
}

// SetBinaryDetection enables or disables binary file detection
func (f *Filters) SetBinaryDetection(enabled bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.binaryDetection = enabled
}

// SetAllowHidden enables or disables inclusion of hidden files
func (f *Filters) SetAllowHidden(allow bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.allowHidden = allow
}

// AddCustomPattern adds a custom regex pattern for filename matching
func (f *Filters) AddCustomPattern(pattern string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	regex, err := regexp.Compile(pattern)
	if err != nil {
		return err
	}

	f.customPatterns = append(f.customPatterns, regex)
	return nil
}

// GetSupportedLanguages returns a list of all supported languages
func (f *Filters) GetSupportedLanguages() []string {
	f.mu.RLock()
	defer f.mu.RUnlock()

	var languages []string
	for name := range f.languages {
		languages = append(languages, name)
	}
	return languages
}

// GetLanguageExtensions returns all extensions for a given language
func (f *Filters) GetLanguageExtensions(langName string) []string {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if lang, ok := f.languages[strings.ToLower(langName)]; ok {
		return append([]string{}, lang.Extensions...)
	}
	return nil
}

// isHiddenFile checks if a filename represents a hidden file
func isHiddenFile(name string) bool {
	return len(name) > 0 && name[0] == '.'
}

// Common file filters

// CreateSourceCodeFilter creates a filter for common source code files
func CreateSourceCodeFilter() *Filters {
	f := NewFilters()

	// Include common source code types
	sourceTypes := []string{"go", "javascript", "typescript", "python", "java", "c", "c++", "rust", "c#", "ruby", "php"}
	for _, t := range sourceTypes {
		f.IncludeType(t)
	}

	// Exclude binary and temporary files
	f.ExcludeExtension(".exe")
	f.ExcludeExtension(".dll")
	f.ExcludeExtension(".so")
	f.ExcludeExtension(".dylib")
	f.ExcludeExtension(".class")
	f.ExcludeExtension(".jar")
	f.ExcludeExtension(".tmp")
	f.ExcludeExtension(".log")

	f.SetBinaryDetection(true)
	f.SetAllowHidden(false)

	return f
}

// CreateTextFileFilter creates a filter for text files only
func CreateTextFileFilter() *Filters {
	f := NewFilters()
	f.SetBinaryDetection(true)
	return f
}
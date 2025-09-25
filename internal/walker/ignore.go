// Package walker provides gitignore support for file system traversal
package walker

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// IgnoreRule represents a single ignore rule
type IgnoreRule struct {
	Pattern   string         // Original pattern
	Regex     *regexp.Regexp // Compiled regex
	Negate    bool           // Whether this is a negation rule (starts with !)
	DirOnly   bool           // Whether this rule applies only to directories (ends with /)
	Anchored  bool           // Whether this rule is anchored to a specific path
	BaseDir   string         // Base directory for this rule
	LineNum   int            // Line number in the ignore file
	Source    string         // Source file path
}

// IgnoreFile represents a .gitignore file and its rules
type IgnoreFile struct {
	Path  string        // Path to the ignore file
	Dir   string        // Directory containing the ignore file
	Rules []*IgnoreRule // Parsed rules from this file
}

// IgnoreManager manages ignore rules across multiple .gitignore files
type IgnoreManager struct {
	files   []*IgnoreFile // All loaded ignore files
	cache   sync.Map      // Cache for ignore decisions
	mu      sync.RWMutex  // Protects files slice
	enabled bool          // Whether ignore rules are enabled
}

// NewIgnoreManager creates a new ignore manager
func NewIgnoreManager() (*IgnoreManager, error) {
	return &IgnoreManager{
		files:   make([]*IgnoreFile, 0),
		enabled: true,
	}, nil
}

// LoadFromPath loads ignore rules from .gitignore and .rgignore files
// starting from the given path and walking up the directory tree
func (im *IgnoreManager) LoadFromPath(root string) error {
	im.mu.Lock()
	defer im.mu.Unlock()

	// Clear existing files
	im.files = im.files[:0]
	im.cache = sync.Map{}

	// Walk up the directory tree looking for ignore files
	current := root
	for {
		// Check for .gitignore
		gitignorePath := filepath.Join(current, ".gitignore")
		if info, err := os.Stat(gitignorePath); err == nil && !info.IsDir() {
			if ignoreFile, err := im.loadIgnoreFile(gitignorePath); err == nil {
				im.files = append(im.files, ignoreFile)
			}
		}

		// Check for .rgignore (ripgrep compatibility)
		rgignorePath := filepath.Join(current, ".rgignore")
		if info, err := os.Stat(rgignorePath); err == nil && !info.IsDir() {
			if ignoreFile, err := im.loadIgnoreFile(rgignorePath); err == nil {
				im.files = append(im.files, ignoreFile)
			}
		}

		// Move up one directory
		parent := filepath.Dir(current)
		if parent == current {
			break // Reached root
		}
		current = parent
	}

	// Also check for global gitignore
	if globalPath := getGlobalGitignore(); globalPath != "" {
		if info, err := os.Stat(globalPath); err == nil && !info.IsDir() {
			if ignoreFile, err := im.loadIgnoreFile(globalPath); err == nil {
				im.files = append(im.files, ignoreFile)
			}
		}
	}

	return nil
}

// loadIgnoreFile loads and parses an ignore file
func (im *IgnoreManager) loadIgnoreFile(path string) (*IgnoreFile, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	ignoreFile := &IgnoreFile{
		Path:  path,
		Dir:   filepath.Dir(path),
		Rules: make([]*IgnoreRule, 0),
	}

	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		rule, err := im.parseRule(line, ignoreFile.Dir, lineNum, path)
		if err != nil {
			// Log error but continue parsing
			continue
		}

		ignoreFile.Rules = append(ignoreFile.Rules, rule)
	}

	return ignoreFile, scanner.Err()
}

// parseRule parses a single ignore rule line
func (im *IgnoreManager) parseRule(pattern, baseDir string, lineNum int, source string) (*IgnoreRule, error) {
	original := pattern

	// Handle comments - should return error as they should be filtered before parseRule
	if strings.HasPrefix(strings.TrimSpace(pattern), "#") {
		return nil, fmt.Errorf("comments should be filtered out before parseRule")
	}

	rule := &IgnoreRule{
		Pattern: original,
		BaseDir: baseDir,
		LineNum: lineNum,
		Source:  source,
	}

	// Handle negation (!)
	if strings.HasPrefix(pattern, "!") {
		rule.Negate = true
		pattern = pattern[1:]
	}

	// Handle directory-only rules (/)
	if strings.HasSuffix(pattern, "/") {
		rule.DirOnly = true
		pattern = strings.TrimSuffix(pattern, "/")
		// Note: For directory patterns, we need to match the directory AND its contents
	}

	// Handle anchored patterns
	if strings.HasPrefix(pattern, "/") {
		rule.Anchored = true
		pattern = pattern[1:] // Remove the leading slash
	} else if strings.Contains(pattern, "/") && !strings.HasPrefix(pattern, "**/") {
		// Only consider it anchored if it contains "/" but doesn't start with "**/",
		// because "**/" patterns are meant to match anywhere in the directory tree
		rule.Anchored = true
	}

	// Convert gitignore pattern to regex
	regexPattern, err := im.patternToRegex(pattern, rule.Anchored)
	if err != nil {
		return nil, err
	}

	// Compile regex
	regex, err := regexp.Compile(regexPattern)
	if err != nil {
		return nil, fmt.Errorf("failed to compile regex '%s': %w", regexPattern, err)
	}

	rule.Regex = regex
	return rule, nil
}

// patternToRegex converts a gitignore pattern to a regular expression
func (im *IgnoreManager) patternToRegex(pattern string, anchored bool) (string, error) {
	// Handle the special pattern **/ first (matches any directory path)
	if pattern == "**" {
		return "^.*$", nil
	}

	// Handle patterns like **/*.ext (should match at any level including root)
	if strings.HasPrefix(pattern, "**/") {
		// Remove the **/ prefix and create a pattern that matches at any level
		suffix := pattern[3:] // Remove "**/"

		// Escape the suffix part
		suffix = regexp.QuoteMeta(suffix)

		// Replace wildcards in the suffix
		suffix = strings.ReplaceAll(suffix, `\*`, "[^/]*")
		suffix = strings.ReplaceAll(suffix, `\?`, "[^/]")

		// Handle character classes
		suffix = strings.ReplaceAll(suffix, `\[`, "[")
		suffix = strings.ReplaceAll(suffix, `\]`, "]")
		suffix = strings.ReplaceAll(suffix, `\!`, "!")

		// Create pattern that matches at any depth (including root)
		return "^(.*/)?" + suffix + "$", nil
	}

	// Handle other patterns normally
	pattern = regexp.QuoteMeta(pattern)

	// Restore gitignore wildcards
	pattern = strings.ReplaceAll(pattern, `\*\*`, ".*")
	pattern = strings.ReplaceAll(pattern, `\*`, "[^/]*")
	pattern = strings.ReplaceAll(pattern, `\?`, "[^/]")

	// Handle character classes [abc] and [!abc]
	pattern = strings.ReplaceAll(pattern, `\[`, "[")
	pattern = strings.ReplaceAll(pattern, `\]`, "]")
	pattern = strings.ReplaceAll(pattern, `\!`, "!")

	// Add anchors
	pattern = "^" + pattern + "$"

	return pattern, nil
}

// ShouldIgnore determines if a file should be ignored based on all loaded rules
func (im *IgnoreManager) ShouldIgnore(relPath string, isDir bool) bool {
	if !im.enabled {
		return false
	}

	// Check cache first
	cacheKey := fmt.Sprintf("%s:%t", relPath, isDir)
	if cached, ok := im.cache.Load(cacheKey); ok {
		return cached.(bool)
	}

	im.mu.RLock()
	files := make([]*IgnoreFile, len(im.files))
	copy(files, im.files)
	im.mu.RUnlock()

	ignored := false

	// Process rules in order (later rules override earlier ones)
	for _, ignoreFile := range files {
		for _, rule := range ignoreFile.Rules {
			if im.ruleMatches(rule, relPath, isDir) {
				ignored = !rule.Negate
			}
		}
	}

	// Cache the result
	im.cache.Store(cacheKey, ignored)
	return ignored
}

// ruleMatches checks if a rule matches the given path
func (im *IgnoreManager) ruleMatches(rule *IgnoreRule, relPath string, isDir bool) bool {
	// Normalize path separators to forward slashes
	path := strings.ReplaceAll(relPath, string(filepath.Separator), "/")

	// For anchored patterns, match against the full path only
	if rule.Anchored {
		// First check direct match
		if rule.Regex.MatchString(path) {
			// For directory-only rules, only match if this is actually a directory
			if rule.DirOnly && !isDir {
				return false
			}
			return true
		}

		// For directory patterns, also check if this path is under the directory
		if rule.DirOnly {
			// Check if any parent directory of this path matches the pattern
			parts := strings.Split(path, "/")
			for i := 1; i <= len(parts); i++ {
				parentPath := strings.Join(parts[:i], "/")
				if rule.Regex.MatchString(parentPath) {
					// If this is the exact path and it's not a directory, don't match
					if i == len(parts) && !isDir {
						continue
					}
					return true
				}
			}
		}

		return false
	}

	// For non-anchored patterns, try matching at each level
	parts := strings.Split(path, "/")

	// Try matching the full path first
	if rule.Regex.MatchString(path) {
		// For directory-only rules, only match if this is actually a directory
		if rule.DirOnly && !isDir {
			return false
		}
		return true
	}

	// Then try each suffix
	for i := 0; i < len(parts); i++ {
		subPath := strings.Join(parts[i:], "/")
		if rule.Regex.MatchString(subPath) {
			return true
		}

		// Also check each individual component for directory matching
		component := parts[i]
		if rule.Regex.MatchString(component) {
			// This component matches, so any file under it should be ignored
			// unless it's a directory-only pattern and we're checking the exact match
			if rule.DirOnly {
				// For directory-only patterns, match if:
				// 1. This is the exact component and it's a directory, OR
				// 2. This is a parent component (i.e., we have more path parts after this)
				if i == len(parts)-1 {
					// This is the last component - only match if it's a directory
					return isDir
				} else {
					// This is a parent component - always match
					return true
				}
			} else {
				// Non-directory-only patterns match files under matching directories
				return true
			}
		}
	}

	return false
}

// AddRule adds a custom ignore rule
func (im *IgnoreManager) AddRule(pattern string) error {
	im.mu.Lock()
	defer im.mu.Unlock()

	rule, err := im.parseRule(pattern, "", 0, "custom")
	if err != nil {
		return err
	}

	// Add to a custom ignore file
	customFile := &IgnoreFile{
		Path:  "custom",
		Dir:   "",
		Rules: []*IgnoreRule{rule},
	}

	im.files = append(im.files, customFile)
	im.cache = sync.Map{} // Clear cache
	return nil
}

// ClearCache clears the ignore decision cache
func (im *IgnoreManager) ClearCache() {
	im.cache = sync.Map{}
}

// SetEnabled enables or disables ignore processing
func (im *IgnoreManager) SetEnabled(enabled bool) {
	im.enabled = enabled
	if !enabled {
		im.cache = sync.Map{} // Clear cache when disabled
	}
}

// GetStats returns statistics about loaded ignore rules
func (im *IgnoreManager) GetStats() map[string]interface{} {
	im.mu.RLock()
	defer im.mu.RUnlock()

	totalRules := 0
	negationRules := 0
	dirOnlyRules := 0
	anchoredRules := 0

	for _, file := range im.files {
		for _, rule := range file.Rules {
			totalRules++
			if rule.Negate {
				negationRules++
			}
			if rule.DirOnly {
				dirOnlyRules++
			}
			if rule.Anchored {
				anchoredRules++
			}
		}
	}

	return map[string]interface{}{
		"total_files":     len(im.files),
		"total_rules":     totalRules,
		"negation_rules":  negationRules,
		"dir_only_rules":  dirOnlyRules,
		"anchored_rules":  anchoredRules,
		"cache_enabled":   im.enabled,
	}
}

// getGlobalGitignore returns the path to the global gitignore file
func getGlobalGitignore() string {
	// Check environment variables
	if path := os.Getenv("GIT_CONFIG_GLOBAL"); path != "" {
		return path
	}

	// Check common locations
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	candidates := []string{
		filepath.Join(home, ".gitignore_global"),
		filepath.Join(home, ".config", "git", "ignore"),
		filepath.Join(home, ".gitignore"),
	}

	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}

	return ""
}

// Common gitignore patterns for different languages and tools
var CommonIgnorePatterns = map[string][]string{
	"go": {
		"*.exe", "*.exe~", "*.dll", "*.so", "*.dylib",
		"*.test", "*.out", "coverage.*", "*.coverprofile", "profile.cov",
		"vendor/", "go.work", "go.work.sum",
	},
	"node": {
		"node_modules/", "npm-debug.log*", "yarn-debug.log*", "yarn-error.log*",
		".npm", ".eslintcache", ".nyc_output", "coverage/", ".coverage",
		"*.tgz", "*.tar.gz", ".cache/",
	},
	"python": {
		"__pycache__/", "*.py[cod]", "*$py.class", "*.so",
		".Python", "build/", "develop-eggs/", "dist/", "downloads/",
		"eggs/", ".eggs/", "lib/", "lib64/", "parts/", "sdist/", "var/",
		"wheels/", "pip-wheel-metadata/", "share/python-wheels/",
		"*.egg-info/", ".installed.cfg", "*.egg", "MANIFEST",
		".env", ".venv", "env/", "venv/", "ENV/", "env.bak/", "venv.bak/",
	},
	"java": {
		"*.class", "*.log", "*.ctxt", ".mtj.tmp/", "*.jar", "*.war", "*.nar",
		"*.ear", "*.zip", "*.tar.gz", "*.rar", "hs_err_pid*",
		"target/", ".mvn/", "mvnw", "mvnw.cmd", ".gradle/", "build/",
		"gradle-app.setting", "!gradle-wrapper.jar", ".gradletasknamecache",
	},
	"rust": {
		"target/", "**/*.rs.bk", "*.pdb", "Cargo.lock",
	},
	"common": {
		".DS_Store", "Thumbs.db", "*.tmp", "*.temp", "*.log",
		".idea/", ".vscode/", "*.swp", "*.swo", "*~",
		".env", ".env.local", ".env.*.local",
	},
}

// AddCommonPatterns adds common ignore patterns for a language
func (im *IgnoreManager) AddCommonPatterns(language string) error {
	patterns, ok := CommonIgnorePatterns[language]
	if !ok {
		return fmt.Errorf("unknown language: %s", language)
	}

	for _, pattern := range patterns {
		if err := im.AddRule(pattern); err != nil {
			return fmt.Errorf("failed to add pattern '%s': %w", pattern, err)
		}
	}

	return nil
}

// ParseRuleForDebug exposes rule parsing for debugging purposes
func (im *IgnoreManager) ParseRuleForDebug(pattern, baseDir string, lineNum int, source string) (*IgnoreRule, error) {
	return im.parseRule(pattern, baseDir, lineNum, source)
}
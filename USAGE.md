# CodeGrep Usage Guide

CodeGrep is a fast semantic code search tool that maintains full compatibility with ripgrep while adding powerful semantic search capabilities powered by tree-sitter.

## Table of Contents

- [Basic Usage](#basic-usage)
- [Semantic Search Features](#semantic-search-features)
- [Index Management](#index-management)
- [Output Formats](#output-formats)
- [Language Support](#language-support)
- [Advanced Examples](#advanced-examples)
- [Performance Tips](#performance-tips)

## Basic Usage

### Regular Search (ripgrep-compatible)

CodeGrep works exactly like ripgrep for regular text search:

```bash
# Basic pattern search
codegrep "func.*main" src/

# Case insensitive search
codegrep -i "error" --type go

# Show line numbers with context
codegrep -n -A 3 "TODO" .

# Count matches per file
codegrep -c "import" --type py

# Search only specific file types
codegrep "class" --type py --type js
```

## Semantic Search Features

### Symbol Search (`--symbols`)

Find symbol definitions (functions, classes, variables, etc.):

```bash
# Find all functions named "main"
codegrep --symbols "main" --type go

# Find any symbol containing "User"
codegrep --symbols "User" src/

# Search for symbols in specific languages
codegrep --symbols "handleRequest" --type go --type ts

# Case-insensitive symbol search
codegrep --symbols -i "parser" .

# With JSON output for detailed information
codegrep --symbols "NewServer" --json
```

**Example Output:**
```
internal/server/server.go:15:func NewServer() *Server {
internal/api/client.go:23:func NewServerClient() *Client {
```

### Reference Search (`--refs`)

Find where symbols are used/referenced:

```bash
# Find all references to "User" type
codegrep --refs "User" --type go

# Find references with context lines
codegrep --refs "logger" -A 2 -B 2

# Count references per file
codegrep --refs "database" -c

# Search references in multiple languages
codegrep --refs "config" --type go --type py --type js
```

**Example Output:**
```
models/user.go:45:    user := &User{Name: name}
services/auth.go:12:  func ValidateUser(u *User) error {
handlers/api.go:67:   users := make([]*User, 0)
```

### Type Search (`--types`)

Find type definitions (structs, classes, interfaces, enums):

```bash
# Find all struct/class definitions named "Config"
codegrep --types "Config" --type go

# Search for interface definitions
codegrep --types "Handler" --type go

# Find enum definitions in Rust
codegrep --types "Status" --type rust

# Search with wildcard patterns
codegrep --types ".*Error" --type go
```

**Example Output:**
```
internal/config/config.go:15:type Config struct {
pkg/server/config.go:8:type ServerConfig struct {
```

### Call Graph Search (`--call-graph`)

Analyze function call relationships:

```bash
# Show what functions call "main"
codegrep --call-graph "main" --type go

# Analyze call patterns with JSON output
codegrep --call-graph "processRequest" --json

# Find call graphs in specific directories
codegrep --call-graph "handleError" internal/
```

## Index Management

CodeGrep uses a semantic index for fast symbol searching. Manage it with these commands:

### Check Index Status
```bash
# View current index statistics
codegrep index status
```

**Example Output:**
```
Semantic Index Status
Index location: /Users/you/.cache/codegrep/index
Index size: 2.1 GB

Content Statistics
Files indexed: 1,234
Symbols indexed: 45,678
Source code size: 12.3 MB
Last updated: 2025-09-25 15:30:00 (2 minutes ago)

Language Distribution
  Go          : 856 files (69.4%)
  Python      : 234 files (19.0%)
  JavaScript  : 144 files (11.6%)

Index Health: ✅ Healthy
```

### Rebuild Index
```bash
# Rebuild index for current directory
codegrep index rebuild

# Rebuild for specific paths
codegrep index rebuild src/ internal/

# Force full rebuild (ignore cache)
codegrep index rebuild --force
```

### Clear Index
```bash
# Remove all index data
codegrep index clear

# Clear and rebuild in one command
codegrep index clear && codegrep index rebuild
```

### Custom Index Location
```bash
# Use custom index path
codegrep --index-path /path/to/index index status
codegrep --index-path /path/to/index --symbols "main"
```

## Output Formats

### Standard Text Output
```bash
# Default colored output
codegrep --symbols "main" --type go

# Without colors
codegrep --symbols "main" --color never

# Show only filenames with matches
codegrep --symbols "User" -l
```

### JSON Output
```bash
# Detailed JSON with metadata
codegrep --symbols "handleRequest" --json
```

**Example JSON Output:**
```json
{
  "type": "match",
  "data": {
    "path": {"text": "internal/api/handler.go"},
    "lines": {"text": "handleRequest"},
    "line_number": 25,
    "symbol_info": {
      "kind": "function",
      "signature": "func handleRequest(w http.ResponseWriter, r *http.Request)",
      "language": "go",
      "scope": "package internal/api"
    }
  }
}
```

### Count Matches
```bash
# Show match count per file
codegrep --symbols "init" -c

# Total count only
codegrep --symbols "error" -c --no-heading
```

## Language Support

CodeGrep supports 8 programming languages with full tree-sitter parsing:

| Language   | Extensions | Symbols Supported |
|------------|------------|------------------|
| Go         | `.go` | functions, methods, structs, interfaces, vars, constants |
| Python     | `.py`, `.pyx`, `.pyi` | functions, classes, methods, variables |
| JavaScript | `.js`, `.mjs`, `.jsx` | functions, classes, variables, exports |
| TypeScript | `.ts`, `.tsx`, `.d.ts` | functions, classes, interfaces, types |
| Rust       | `.rs` | functions, structs, enums, traits, impls |
| C          | `.c`, `.h` | functions, structs, enums, typedefs |
| C++        | `.cpp`, `.cc`, `.cxx`, `.hpp` | functions, classes, namespaces |
| Java       | `.java` | methods, classes, interfaces, enums |

### Language-Specific Examples

**Go:**
```bash
codegrep --symbols "NewHandler" --type go
codegrep --refs "context.Context" --type go
codegrep --types "Server" --type go
```

**Python:**
```bash
codegrep --symbols "__init__" --type py
codegrep --refs "self" --type py
codegrep --types "DataProcessor" --type py
```

**TypeScript:**
```bash
codegrep --symbols "interface" --type ts
codegrep --refs "Promise" --type ts
codegrep --types "Component" --type ts
```

## Advanced Examples

### Combining Search Types
```bash
# Find symbols and their references
codegrep --symbols "User" --type go && codegrep --refs "User" --type go

# Search multiple patterns
codegrep --symbols "Handle.*" --type go | head -20
```

### Cross-Language Analysis
```bash
# Find similar patterns across languages
codegrep --symbols "Config" --type go --type py --type js

# API analysis across frontend/backend
codegrep --refs "api/v1" --type js --type ts
codegrep --symbols "v1.*Handler" --type go
```

### Large Codebase Analysis
```bash
# Performance-focused search
codegrep --symbols "main" --threads 8 --max-count 50

# Directory-specific searches
codegrep --symbols "test" internal/ --type go
codegrep --refs "import" frontend/ --type js --type ts
```

### Integration with Other Tools
```bash
# Pipe to other tools
codegrep --symbols "Error" --json | jq '.data.symbol_info.signature'

# Count unique symbol types
codegrep --symbols ".*" --json | jq -r '.data.symbol_info.kind' | sort | uniq -c

# Export symbol list
codegrep --symbols ".*" -l > symbols.txt
```

## Performance Tips

### Optimize Index Performance
```bash
# Build index with multiple threads
codegrep index rebuild --threads 8

# Index only what you need
codegrep index rebuild src/ internal/ --exclude vendor/
```

### Search Performance
```bash
# Use file type filters for faster searches
codegrep --symbols "main" --type go  # Fast
codegrep --symbols "main"  # Slower (searches all files)

# Limit search scope
codegrep --symbols "handler" internal/api/  # Fast
codegrep --symbols "handler" .  # Slower
```

### Memory Usage
```bash
# For large repositories, use custom index location
export CODEGREP_INDEX_PATH=/fast-ssd/codegrep-index
codegrep index rebuild

# Or use temporary index for one-time analysis
codegrep --index-path /tmp/analysis-index --symbols "main"
```

## Configuration

### Environment Variables
```bash
# Default index location
export CODEGREP_INDEX_PATH=/path/to/index

# Default thread count
export CODEGREP_THREADS=8

# Enable debug logging
export CODEGREP_DEBUG=1
```

### Ignore Files
CodeGrep respects standard ignore files:
- `.gitignore`
- `.ignore`
- `.rgignore`
- `.codegrepignore`

### Common Workflows

**Code Review Workflow:**
```bash
# Find all TODOs and FIXMEs
codegrep -i "todo|fixme" --type go --type py

# Analyze new API endpoints
codegrep --symbols ".*Handler" internal/api/

# Check error handling patterns
codegrep --refs "error" --type go -A 2
```

**Refactoring Workflow:**
```bash
# Find all usages before renaming
codegrep --refs "OldFunctionName" --type go

# Analyze type dependencies
codegrep --types "UserModel" --json | jq '.data.path.text' | sort | uniq

# Find similar patterns to refactor
codegrep --symbols "Process.*" --type go
```

**Learning Codebase Workflow:**
```bash
# Understand main entry points
codegrep --symbols "main" --type go

# Find all interfaces
codegrep --types ".*Interface" --type go

# Explore API endpoints
codegrep --symbols ".*Handler|.*Controller" --type go --type py
```

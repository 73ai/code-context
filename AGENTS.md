### Go Code Style Guidelines
- Follow standard Go conventions from Effective Go
- Use gofmt/goimports for code formatting (mandatory pre-commit)
- Error handling: Always wrap errors with context using fmt.Errorf
- Naming: CamelCase for exported symbols, camelCase for non-exported
- Package organization: One package per directory, package name matches directory
- Use type definitions over string constants for enumerations
- Self-documenting code: Prefer descriptive names over explanatory comments
- Do not add json tags to every struct, we only needs json tags when we need to serialize/deserialize the struct to/from json. ex. api handlers, external json api processing etc. 
- Do not use `interface{}` in the backend, use `any` instead
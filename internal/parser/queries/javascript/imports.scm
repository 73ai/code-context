; Import declarations
(import_statement
  source: (string) @source) @import

; Named imports
(import_statement
  (import_clause
    (named_imports
      (import_specifier
        name: (identifier) @imported_name
        alias: (identifier)? @alias)*)) @named_imports
  source: (string) @source) @import.named

; Default imports
(import_statement
  (import_clause
    (identifier) @default_import_name)
  source: (string) @source) @import.default

; Namespace imports
(import_statement
  (import_clause
    (namespace_import
      (identifier) @namespace_name))
  source: (string) @source) @import.namespace

; Mixed imports (default + named)
(import_statement
  (import_clause
    (identifier) @default_name
    (named_imports
      (import_specifier
        name: (identifier) @named_import)*))
  source: (string) @source) @import.mixed

; Dynamic imports
(call_expression
  function: "import" @import_function
  arguments: (arguments
    (string) @dynamic_source)) @import.dynamic

; Export declarations
(export_statement
  declaration: (_) @exported_declaration) @export

; Named exports
(export_statement
  (export_clause
    (export_specifier
      name: (identifier) @exported_name
      alias: (identifier)? @export_alias)*)) @export.named

; Default exports
(export_statement
  "default" @default_export
  declaration: (_) @default_exported_declaration) @export.default

; Re-exports
(export_statement
  source: (string) @re_export_source) @export.re_export
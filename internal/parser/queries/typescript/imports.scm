; Extend JavaScript imports with TypeScript-specific imports

; Type-only imports
(import_statement
  "type" @type_only
  (import_clause
    (named_imports
      (import_specifier
        name: (identifier) @type_import_name
        alias: (identifier)? @type_import_alias)*))
  source: (string) @type_import_source) @import.type_only

; Type-only default imports
(import_statement
  "type" @type_only
  (import_clause
    (identifier) @type_default_import)
  source: (string) @type_default_source) @import.type_only.default

; Import equals (CommonJS style)
(import_assignment
  name: (identifier) @import_equals_name
  value: (external_module_reference
    (string) @import_equals_source)) @import.equals

; Import equals with namespace
(import_assignment
  name: (identifier) @import_equals_namespace_name
  value: (qualified_name) @import_equals_namespace_value) @import.equals.namespace

; Export type declarations
(export_statement
  "type" @export_type
  declaration: (_) @exported_type_declaration) @export.type_only

; Export type with source
(export_statement
  "type" @export_type
  (export_clause
    (export_specifier
      name: (identifier) @exported_type_name
      alias: (identifier)? @exported_type_alias)*)
  source: (string) @export_type_source) @export.type_only.named

; Namespace declarations
(module_declaration
  name: (identifier) @namespace_name
  body: (statement_block) @namespace_body) @namespace

; Module declarations with string names
(module_declaration
  name: (string) @module_string_name
  body: (statement_block) @module_string_body) @module

; Ambient module declarations
(ambient_declaration
  "declare" @declare
  declaration: (module_declaration
    name: (_) @ambient_module_name
    body: (statement_block) @ambient_module_body)) @module.ambient

; Triple-slash directives
(comment) @triple_slash_comment

; Reference directives - these need special handling as they're in comments
; /// <reference path="..." />
; /// <reference types="..." />
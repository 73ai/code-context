; Import declarations
(import_declaration
  (import_spec
    name: (package_identifier)? @alias
    path: (interpreted_string_literal) @path)) @import

; Import with dot notation
(import_declaration
  (import_spec
    name: "." @dot_import
    path: (interpreted_string_literal) @path)) @import.dot

; Import with blank identifier
(import_declaration
  (import_spec
    name: "_" @blank_import
    path: (interpreted_string_literal) @path)) @import.blank

; Package declaration
(package_clause
  (package_identifier) @package_name) @package
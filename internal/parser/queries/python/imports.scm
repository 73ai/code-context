; Import statements
(import_statement
  name: (dotted_name) @module_name) @import

; Import with alias
(import_statement
  name: (aliased_import
    name: (dotted_name) @module_name
    alias: (identifier) @alias)) @import.alias

; From import statements
(import_from_statement
  module_name: (dotted_name) @module_name
  name: (dotted_name) @imported_name) @import.from

; From import with alias
(import_from_statement
  module_name: (dotted_name) @module_name
  name: (aliased_import
    name: (dotted_name) @imported_name
    alias: (identifier) @alias)) @import.from.alias

; Star imports
(import_from_statement
  module_name: (dotted_name) @module_name
  name: (wildcard_import)) @import.star

; Relative imports
(import_from_statement
  module_name: (relative_import
    (import_prefix) @prefix
    (dotted_name)? @module_name)
  name: (dotted_name) @imported_name) @import.relative
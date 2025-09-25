; Use declarations
(use_declaration
  argument: (identifier) @simple_use_name) @import

; Use declarations with paths
(use_declaration
  argument: (scoped_identifier
    path: (identifier) @use_path_root
    name: (identifier) @use_path_name)) @import.scoped

; Use declarations with use trees
(use_declaration
  argument: (use_as_clause
    path: (_) @use_as_path
    alias: (identifier) @use_as_alias)) @import.alias

; Use declarations with wildcards
(use_declaration
  argument: (use_wildcard
    (scoped_identifier) @wildcard_path)) @import.wildcard

; Use declarations with lists
(use_declaration
  argument: (use_list
    (use_list
      (identifier) @use_list_item)*)) @import.list

; External crate declarations
(extern_crate_declaration
  name: (identifier) @extern_crate_name
  alias: (identifier)? @extern_crate_alias) @import.extern_crate

; Module declarations
(mod_item
  name: (identifier) @module_name
  body: (declaration_list)? @module_body) @module

; Module declarations with external files
(mod_item
  name: (identifier) @external_module_name) @module.external

; Pub use (re-exports)
(use_declaration
  visibility: (visibility_modifier) @pub_use_visibility
  argument: (_) @pub_use_argument) @export

; Macro use
(use_declaration
  argument: (scoped_identifier
    path: (identifier) @macro_path
    name: (identifier) @macro_name)) @import.macro

; Absolute paths
(use_declaration
  argument: (scoped_identifier
    path: "crate" @crate_root
    name: (identifier) @crate_item_name)) @import.crate

; Standard library paths
(use_declaration
  argument: (scoped_identifier
    path: "std" @std_root
    name: (_) @std_item_name)) @import.std

; Super and self paths
(use_declaration
  argument: (scoped_identifier
    path: "super" @super_root
    name: (identifier) @super_item_name)) @import.super

(use_declaration
  argument: (scoped_identifier
    path: "self" @self_root
    name: (identifier) @self_item_name)) @import.self_path
; Type declarations
(type_declaration
  (type_spec
    name: (type_identifier) @name
    type: (_) @type)) @type_declaration

; Struct types
(struct_type
  (field_declaration_list
    (field_declaration
      name: (field_identifier) @field_name
      type: (_) @field_type
      tag: (raw_string_literal)? @field_tag)*)) @struct

; Interface types
(interface_type
  (method_spec_list
    (method_spec
      name: (field_identifier) @method_name
      parameters: (parameter_list) @method_params
      result: (parameter_list)? @method_return)*)) @interface

; Type aliases
(type_declaration
  (type_spec
    name: (type_identifier) @name
    type: (_) @aliased_type)) @type_alias

; Generic type constraints
(type_constraint
  (type_elem
    type: (_) @constraint_type)*) @constraint
; Variable declarations
(var_declaration
  (var_spec
    name: (identifier) @name
    type: (_)? @type
    value: (_)? @value)) @variable

; Short variable declarations
(short_var_declaration
  left: (expression_list
    (identifier) @name)
  right: (expression_list) @value) @variable

; Constant declarations
(const_declaration
  (const_spec
    name: (identifier) @name
    type: (_)? @type
    value: (_)? @value)) @constant

; Field declarations in structs
(field_declaration
  name: (field_identifier) @name
  type: (_) @type
  tag: (raw_string_literal)? @tag) @field

; Parameters in functions
(parameter_declaration
  name: (identifier) @name
  type: (_) @type) @parameter
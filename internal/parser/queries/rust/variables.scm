; Let bindings
(let_declaration
  pattern: (identifier) @name
  type: (type_annotation)? @type
  value: (_)? @value) @variable

; Mutable let bindings
(let_declaration
  "mut" @mut
  pattern: (identifier) @mut_name
  type: (type_annotation)? @mut_type
  value: (_)? @mut_value) @variable.mutable

; Static variables
(static_item
  name: (identifier) @static_name
  type: (type_annotation) @static_type
  value: (_) @static_value) @variable.static

; Mutable static variables
(static_item
  "mut" @static_mut
  name: (identifier) @static_mut_name
  type: (type_annotation) @static_mut_type
  value: (_) @static_mut_value) @variable.static.mutable

; Constant items
(const_item
  name: (identifier) @const_name
  type: (type_annotation) @const_type
  value: (_) @const_value) @constant

; Function parameters
(parameters
  (parameter
    pattern: (identifier) @param_name
    type: (type_annotation) @param_type) @parameter)

; Self parameters
(parameters
  (self_parameter) @self_param) @parameter.self

; Destructuring patterns
(let_declaration
  pattern: (tuple_pattern
    (identifier) @destructured_name)*
  value: (_) @destructured_value) @variable.destructured

; Struct patterns
(let_declaration
  pattern: (struct_pattern
    type: (type_identifier) @struct_pattern_type
    (field_pattern
      name: (field_identifier) @struct_field_name
      pattern: (identifier) @struct_field_pattern)*) @struct_pattern_binding) @variable.struct_pattern

; Reference patterns
(let_declaration
  pattern: (ref_pattern
    pattern: (identifier) @ref_name)) @variable.reference

; For loop variables
(for_expression
  pattern: (identifier) @for_var_name
  value: (_) @for_iterable) @variable.for_loop

; While let patterns
(while_expression
  condition: (let_condition
    pattern: (identifier) @while_let_name
    value: (_) @while_let_value)) @variable.while_let

; If let patterns
(if_expression
  condition: (let_condition
    pattern: (identifier) @if_let_name
    value: (_) @if_let_value)) @variable.if_let

; Match arms
(match_expression
  (match_arm
    pattern: (identifier) @match_var_name)) @variable.match

; Closure parameters
(closure_expression
  parameters: (closure_parameters
    (identifier) @closure_param_name)*) @parameter.closure
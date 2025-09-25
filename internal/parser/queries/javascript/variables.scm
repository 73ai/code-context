; Variable declarations
(variable_declaration
  (variable_declarator
    name: (identifier) @name
    value: (_)? @value)) @variable

; Let declarations
(lexical_declaration
  "let" @let
  (variable_declarator
    name: (identifier) @name
    value: (_)? @value)) @variable.let

; Const declarations
(lexical_declaration
  "const" @const
  (variable_declarator
    name: (identifier) @name
    value: (_) @value)) @variable.const

; Destructuring assignments
(variable_declaration
  (variable_declarator
    name: (array_pattern
      (identifier) @destructured_name)*
    value: (_) @destructured_value)) @variable.destructuring

(variable_declaration
  (variable_declarator
    name: (object_pattern
      (shorthand_property_identifier_pattern) @destructured_name)*
    value: (_) @destructured_value)) @variable.destructuring

; Assignment expressions
(assignment_expression
  left: (identifier) @name
  right: (_) @value) @variable.assignment

; Function parameters
(formal_parameters
  (identifier) @parameter_name) @parameter

; Rest parameters
(formal_parameters
  (rest_parameter
    (identifier) @rest_param_name)) @parameter.rest

; Default parameters
(formal_parameters
  (assignment_pattern
    left: (identifier) @default_param_name
    right: (_) @default_value)) @parameter.default

; For loop variables
(for_statement
  init: (variable_declaration
    (variable_declarator
      name: (identifier) @loop_var_name))) @variable.loop

; For-in loop variables
(for_in_statement
  left: (variable_declaration
    (variable_declarator
      name: (identifier) @for_in_var_name))) @variable.for_in

; For-of loop variables
(for_of_statement
  left: (variable_declaration
    (variable_declarator
      name: (identifier) @for_of_var_name))) @variable.for_of
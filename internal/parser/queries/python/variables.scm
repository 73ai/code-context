; Variable assignments
(assignment
  left: (identifier) @name
  right: (_) @value) @variable

; Multiple assignment
(assignment
  left: (pattern_list
    (identifier) @name)
  right: (_) @value) @variable

; Augmented assignment (+=, -=, etc.)
(augmented_assignment
  left: (identifier) @name
  right: (_) @value) @variable.augmented

; Global declarations
(global_statement
  (identifier) @name) @variable.global

; Nonlocal declarations
(nonlocal_statement
  (identifier) @name) @variable.nonlocal

; For loop variables
(for_statement
  left: (identifier) @name
  right: (_) @iterable) @variable.loop

; With statement variables
(with_statement
  (with_clause
    (as_pattern
      (identifier) @name))) @variable.with

; Function parameters
(parameters
  (identifier) @parameter_name) @parameter

; Keyword-only parameters
(parameters
  (keyword_separator)
  (identifier) @kwonly_parameter) @parameter.kwonly

; Default parameters
(parameters
  (default_parameter
    name: (identifier) @default_param_name
    value: (_) @default_value)) @parameter.default
; Function declarations
(function_declaration
  name: (identifier) @name
  parameters: (formal_parameters) @parameters
  body: (statement_block) @body) @function

; Arrow functions
(arrow_function
  parameter: (identifier) @parameter
  body: (_) @body) @function.arrow

; Arrow functions with parameters
(arrow_function
  parameters: (formal_parameters) @parameters
  body: (_) @body) @function.arrow

; Function expressions
(function_expression
  name: (identifier)? @name
  parameters: (formal_parameters) @parameters
  body: (statement_block) @body) @function.expression

; Method definitions
(method_definition
  name: (property_identifier) @name
  parameters: (formal_parameters) @parameters
  body: (statement_block) @body) @method

; Async functions
(function_declaration
  "async" @async
  name: (identifier) @name
  parameters: (formal_parameters) @parameters
  body: (statement_block) @body) @function.async

; Generator functions
(function_declaration
  "function" @function_keyword
  "*" @generator
  name: (identifier) @name
  parameters: (formal_parameters) @parameters
  body: (statement_block) @body) @function.generator

; Object method shorthand
(pair
  key: (property_identifier) @method_name
  value: (function_expression
    parameters: (formal_parameters) @method_parameters
    body: (statement_block) @method_body)) @method.shorthand
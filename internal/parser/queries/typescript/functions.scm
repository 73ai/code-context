; Extend JavaScript functions with TypeScript features
; Include all JavaScript function patterns
(function_declaration
  name: (identifier) @name
  parameters: (formal_parameters) @parameters
  return_type: (type_annotation)? @return_type
  body: (statement_block) @body) @function

; Arrow functions with type annotations
(arrow_function
  parameters: (formal_parameters) @parameters
  return_type: (type_annotation)? @return_type
  body: (_) @body) @function.arrow

; Method definitions with type annotations
(method_definition
  name: (property_identifier) @name
  parameters: (formal_parameters) @parameters
  return_type: (type_annotation)? @return_type
  body: (statement_block) @body) @method

; Function overloads
(function_signature
  name: (identifier) @overload_name
  parameters: (formal_parameters) @overload_parameters
  return_type: (type_annotation) @overload_return_type) @function.overload

; Generic functions
(function_declaration
  type_parameters: (type_parameters) @type_params
  name: (identifier) @generic_function_name
  parameters: (formal_parameters) @generic_parameters
  return_type: (type_annotation)? @generic_return_type
  body: (statement_block) @generic_body) @function.generic

; Abstract methods
(abstract_method_signature
  name: (property_identifier) @abstract_method_name
  parameters: (formal_parameters) @abstract_parameters
  return_type: (type_annotation)? @abstract_return_type) @method.abstract

; Async functions with types
(function_declaration
  "async" @async
  name: (identifier) @async_name
  parameters: (formal_parameters) @async_parameters
  return_type: (type_annotation)? @async_return_type
  body: (statement_block) @async_body) @function.async

; Constructor signatures
(construct_signature
  parameters: (formal_parameters) @constructor_parameters
  return_type: (type_annotation)? @constructor_return_type) @constructor.signature
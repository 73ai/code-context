; Function items
(function_item
  name: (identifier) @name
  parameters: (parameters) @parameters
  return_type: (type_annotation)? @return_type
  body: (block) @body) @function

; Associated functions (static methods)
(impl_item
  (function_item
    name: (identifier) @associated_function_name
    parameters: (parameters) @associated_function_parameters
    body: (block) @associated_function_body) @associated_function)

; Methods (functions with self)
(impl_item
  (function_item
    name: (identifier) @method_name
    parameters: (parameters
      (self_parameter) @self_param) @method_parameters
    body: (block) @method_body) @method)

; Generic functions
(function_item
  (generic_parameters) @generic_params
  name: (identifier) @generic_function_name
  parameters: (parameters) @generic_function_parameters
  body: (block) @generic_function_body) @function.generic

; Async functions
(function_item
  "async" @async
  name: (identifier) @async_function_name
  parameters: (parameters) @async_function_parameters
  body: (block) @async_function_body) @function.async

; Unsafe functions
(function_item
  "unsafe" @unsafe
  name: (identifier) @unsafe_function_name
  parameters: (parameters) @unsafe_function_parameters
  body: (block) @unsafe_function_body) @function.unsafe

; External functions
(extern_crate_declaration
  (function_item
    name: (identifier) @extern_function_name
    parameters: (parameters) @extern_function_parameters)) @function.extern

; Closures
(closure_expression
  parameters: (closure_parameters) @closure_parameters
  body: (_) @closure_body) @closure

; Function pointers
(function_type
  parameters: (parameters) @function_pointer_parameters
  return_type: (type_annotation)? @function_pointer_return) @function_pointer
; Function definitions
(function_definition
  name: (identifier) @name
  parameters: (parameters) @parameters
  return_type: (type)? @return_type
  body: (block) @body) @function

; Async function definitions
(function_definition
  "async" @async
  name: (identifier) @name
  parameters: (parameters) @parameters
  return_type: (type)? @return_type
  body: (block) @body) @function.async

; Lambda functions
(lambda
  parameters: (lambda_parameters)? @parameters
  body: (_) @body) @function.lambda

; Method definitions (functions inside classes)
(class_definition
  body: (block
    (function_definition
      name: (identifier) @method_name
      parameters: (parameters) @method_parameters
      body: (block) @method_body) @method))
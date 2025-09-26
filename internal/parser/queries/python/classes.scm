; Class definitions
(class_definition
  name: (identifier) @name
  superclasses: (argument_list)? @superclasses
  body: (block) @body) @class

; Class methods
(class_definition
  body: (block
    (function_definition
      name: (identifier) @method_name
      parameters: (parameters) @method_parameters) @method))

; Class variables
(class_definition
  body: (block
    (assignment
      left: (identifier) @class_var_name
      right: (_) @class_var_value) @class_variable))

; Decorators on classes
(decorated_definition
  (decorator
    (dotted_name) @decorator_name) @decorator
  definition: (class_definition
    name: (identifier) @class_name) @decorated_class)
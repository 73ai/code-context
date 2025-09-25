; Class declarations
(class_declaration
  name: (identifier) @name
  superclass: (class_heritage)? @superclass
  body: (class_body) @body) @class

; Class expressions
(class_expression
  name: (identifier)? @name
  superclass: (class_heritage)? @superclass
  body: (class_body) @body) @class.expression

; Class methods
(class_body
  (method_definition
    name: (property_identifier) @method_name
    parameters: (formal_parameters) @method_parameters
    body: (statement_block) @method_body) @method)

; Static methods
(class_body
  (method_definition
    "static" @static
    name: (property_identifier) @static_method_name
    parameters: (formal_parameters) @static_method_parameters
    body: (statement_block) @static_method_body) @method.static)

; Constructor methods
(class_body
  (method_definition
    name: "constructor" @constructor_name
    parameters: (formal_parameters) @constructor_parameters
    body: (statement_block) @constructor_body) @constructor)

; Getters and setters
(class_body
  (method_definition
    "get" @getter
    name: (property_identifier) @getter_name
    body: (statement_block) @getter_body) @method.getter)

(class_body
  (method_definition
    "set" @setter
    name: (property_identifier) @setter_name
    parameters: (formal_parameters) @setter_parameters
    body: (statement_block) @setter_body) @method.setter)

; Class fields (ES2022)
(class_body
  (field_definition
    property: (property_identifier) @field_name
    value: (_)? @field_value) @field)
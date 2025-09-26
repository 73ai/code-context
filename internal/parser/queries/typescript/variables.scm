; Extend JavaScript variables with TypeScript type annotations

; Variable declarations with type annotations
(variable_declaration
  (variable_declarator
    name: (identifier) @name
    type: (type_annotation) @type
    value: (_)? @value)) @variable.typed

; Let declarations with types
(lexical_declaration
  "let" @let
  (variable_declarator
    name: (identifier) @name
    type: (type_annotation) @type
    value: (_)? @value)) @variable.let.typed

; Const declarations with types
(lexical_declaration
  "const" @const
  (variable_declarator
    name: (identifier) @name
    type: (type_annotation) @type
    value: (_) @value)) @variable.const.typed

; Function parameters with type annotations
(formal_parameters
  (required_parameter
    pattern: (identifier) @param_name
    type: (type_annotation) @param_type)) @parameter.typed

; Optional parameters
(formal_parameters
  (optional_parameter
    pattern: (identifier) @optional_param_name
    type: (type_annotation) @optional_param_type)) @parameter.optional

; Rest parameters with types
(formal_parameters
  (rest_parameter
    pattern: (identifier) @rest_param_name
    type: (type_annotation) @rest_param_type)) @parameter.rest.typed

; Property declarations in classes
(class_body
  (property_definition
    name: (property_identifier) @property_name
    type: (type_annotation)? @property_type
    value: (_)? @property_value)) @property

; Static property declarations
(class_body
  (property_definition
    "static" @static
    name: (property_identifier) @static_property_name
    type: (type_annotation)? @static_property_type
    value: (_)? @static_property_value)) @property.static

; Abstract property declarations
(class_body
  (abstract_property_signature
    name: (property_identifier) @abstract_property_name
    type: (type_annotation) @abstract_property_type)) @property.abstract

; Readonly property declarations
(class_body
  (property_definition
    "readonly" @readonly
    name: (property_identifier) @readonly_property_name
    type: (type_annotation)? @readonly_property_type
    value: (_)? @readonly_property_value)) @property.readonly

; Accessibility modifiers (public, private, protected)
(class_body
  (property_definition
    (accessibility_modifier) @access_modifier
    name: (property_identifier) @access_property_name
    type: (type_annotation)? @access_property_type)) @property.access

; Declare statements
(ambient_declaration
  "declare" @declare
  declaration: (variable_statement
    (variable_declarator
      name: (identifier) @declare_var_name
      type: (type_annotation) @declare_var_type))) @variable.declare
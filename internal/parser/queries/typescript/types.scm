; Type alias declarations
(type_alias_declaration
  name: (type_identifier) @name
  value: (_) @type_value) @type_alias

; Interface declarations
(interface_declaration
  name: (type_identifier) @name
  type_parameters: (type_parameters)? @interface_type_params
  body: (object_type) @interface_body) @interface

; Interface properties
(interface_declaration
  body: (object_type
    (property_signature
      name: (property_identifier) @property_name
      type: (type_annotation) @property_type) @property))

; Interface methods
(interface_declaration
  body: (object_type
    (method_signature
      name: (property_identifier) @method_name
      parameters: (formal_parameters) @method_parameters
      return_type: (type_annotation)? @method_return_type) @interface_method))

; Enum declarations
(enum_declaration
  name: (identifier) @enum_name
  body: (enum_body
    (property_identifier) @enum_member)*) @enum

; Enum with values
(enum_declaration
  body: (enum_body
    (enum_assignment
      name: (property_identifier) @enum_value_name
      value: (_) @enum_value)*)) @enum

; Generic type parameters
(type_parameters
  (type_parameter
    name: (type_identifier) @type_param_name
    constraint: (type_annotation)? @type_param_constraint)*) @type_parameters

; Mapped types
(mapped_type_clause
  name: (type_identifier) @mapped_type_name
  type: (_) @mapped_type_value) @mapped_type

; Conditional types
(conditional_type
  left: (_) @condition_left
  right: (_) @condition_right
  consequence: (_) @condition_true
  alternative: (_) @condition_false) @conditional_type

; Utility types
(generic_type
  name: (type_identifier) @utility_type_name
  type_arguments: (type_arguments) @utility_type_args) @utility_type

; Index signatures
(index_signature
  parameter: (identifier) @index_param_name
  type: (type_annotation) @index_param_type
  return_type: (type_annotation) @index_return_type) @index_signature
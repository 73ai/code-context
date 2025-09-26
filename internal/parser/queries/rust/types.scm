; Struct declarations
(struct_item
  name: (type_identifier) @name
  body: (field_declaration_list
    (field_declaration
      name: (field_identifier) @field_name
      type: (_) @field_type)*)) @struct

; Tuple structs
(struct_item
  name: (type_identifier) @tuple_struct_name
  body: (ordered_field_declaration_list
    (ordered_field_declaration
      type: (_) @tuple_field_type)*)) @struct.tuple

; Unit structs
(struct_item
  name: (type_identifier) @unit_struct_name) @struct.unit

; Enum declarations
(enum_item
  name: (type_identifier) @enum_name
  body: (enum_variant_list
    (enum_variant
      name: (identifier) @variant_name)*)) @enum

; Enum variants with fields
(enum_item
  body: (enum_variant_list
    (enum_variant
      name: (identifier) @variant_field_name
      body: (field_declaration_list
        (field_declaration
          name: (field_identifier) @enum_field_name
          type: (_) @enum_field_type)*)) @enum_variant.struct))

; Enum variants with tuple fields
(enum_item
  body: (enum_variant_list
    (enum_variant
      name: (identifier) @variant_tuple_name
      body: (ordered_field_declaration_list
        (ordered_field_declaration
          type: (_) @enum_tuple_field_type)*)) @enum_variant.tuple))

; Type aliases
(type_item
  name: (type_identifier) @type_alias_name
  type: (_) @aliased_type) @type_alias

; Trait declarations
(trait_item
  name: (type_identifier) @trait_name
  body: (declaration_list) @trait_body) @trait

; Trait methods
(trait_item
  body: (declaration_list
    (function_signature_item
      name: (identifier) @trait_method_name
      parameters: (parameters) @trait_method_parameters
      return_type: (type_annotation)? @trait_method_return_type) @trait_method))

; Associated types in traits
(trait_item
  body: (declaration_list
    (associated_type
      name: (type_identifier) @associated_type_name
      bounds: (trait_bounds)? @associated_type_bounds) @associated_type))

; Implementation blocks
(impl_item
  trait: (type_identifier)? @impl_trait
  type: (_) @impl_type
  body: (declaration_list) @impl_body) @impl

; Generic parameters
(generic_parameters
  (type_parameter
    name: (type_identifier) @generic_param_name
    bounds: (trait_bounds)? @generic_param_bounds)*) @generic_parameters

; Where clauses
(where_clause
  (where_predicate
    left: (_) @where_left
    bounds: (trait_bounds) @where_bounds)*) @where_clause

; Union types (unsafe)
(union_item
  name: (type_identifier) @union_name
  body: (field_declaration_list) @union_body) @union
(
  (comment)* @doc
  .
  (function_declaration
    name: (identifier) @name) @definition.function
  (#strip! @doc "^//\\s*")
  (#select-adjacent! @doc @definition.function)
)

(
  (comment)* @doc
  .
  (method_declaration
    receiver: (parameter_list (parameter_declaration type: (pointer_type (type_identifier) @scope)))
    name: (field_identifier) @name) @definition.method
  (#strip! @doc "^//\\s*")
  (#select-adjacent! @doc @definition.method)
)

(
  (comment)* @doc
  .
  (method_declaration
    receiver: (parameter_list (parameter_declaration type: (type_identifier) @scope))
    name: (field_identifier) @name) @definition.method
  (#strip! @doc "^//\\s*")
  (#select-adjacent! @doc @definition.method)
)

(
  (comment)* @doc
  .
  (type_declaration
    (type_spec
      name: (type_identifier) @scope
      type: (struct_type
        (field_declaration_list
          (field_declaration
            name: (field_identifier) @name) @definition.field))))
  (#strip! @doc "^//\\s*")
  (#select-adjacent! @doc @definition.field)
)

(call_expression
  function: [
    (identifier) @name
    (parenthesized_expression (identifier) @name)
    (selector_expression field: (field_identifier) @name)
    (parenthesized_expression (selector_expression field: (field_identifier) @name))
  ]) @reference.call

(call_expression
  function: [
    (selector_expression operand: (identifier) @name)
    (parenthesized_expression (selector_expression operand: (identifier) @name))
  ])

(
  (comment)* @doc
  .
  (package_clause "package" (package_identifier) @name) @definition.module
  (#strip! @doc "^//\\s*")
  (#select-adjacent! @doc @definition.module)
)

(import_declaration (import_spec path: (interpreted_string_literal (interpreted_string_literal_content) @name))) @definition.import

(
  (comment)* @doc
  .
  (var_declaration (var_spec name: (identifier) @name)) @definition.variable
  (#strip! @doc "^//\\s*")
  (#select-adjacent! @doc @definition.variable)
)

(
  (comment)* @doc
  .
  (const_declaration (const_spec name: (identifier) @name)) @definition.constant
  (#strip! @doc "^//\\s*")
  (#select-adjacent! @doc @definition.constant)
)

(
  (comment)* @doc
  .
  (type_declaration (type_spec name: (type_identifier) @name type: (struct_type))) @definition.class
  (#strip! @doc "^//\\s*")
  (#select-adjacent! @doc @definition.class)
)

(
  (comment)* @doc
  .
  (type_declaration (type_spec name: (type_identifier) @name type: (interface_type))) @definition.interface
  (#strip! @doc "^//\\s*")
  (#select-adjacent! @doc @definition.interface)
)

(
  (comment)* @doc
  .
  (type_declaration (type_spec name: (type_identifier) @name type: [(map_type) (channel_type) (slice_type) (array_type) (pointer_type) (type_identifier)])) @definition.type
  (#strip! @doc "^//\\s*")
  (#select-adjacent! @doc @definition.type)
)

(method_elem name: (field_identifier) @name) @definition.method

(argument_list (identifier) @name)

(type_spec name: (type_identifier) @name) @definition.type

(type_identifier) @name @reference.type
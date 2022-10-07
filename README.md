# go-jsonschema

A [JSON schema] code generator for Go.

## Usage

    jsonschemagen -s <schema> -o <output>

One Go type per definition will be generated.

- `int64` is used for `"type": "integer"`.
- `json.Number` is used for `"type": "number"`.
- Go structs are generated for objects with `"additionalProperties": false`.
- `json.RawMessage` is used when a value can have multiple types. Helpers are
  generated for `allOf`, `anyOf`, `oneOf`, `then`, `else` and `dependantSchemas`
  which are references.

## License

MIT

[JSON schema]: https://json-schema.org/

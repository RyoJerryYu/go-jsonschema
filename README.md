# go-jsonschema

A [JSON schema] code generator for Go.

JSON schema draft 2020-12 is supported.

## Usage

    jsonschemagen -s <schema> -o <output>

One Go type per definition will be generated.

- `int64` is used for `"type": "integer"`.
- `json.Number` is used for `"type": "number"`.
- Go structs are generated for objects with `"additionalProperties": false`.
- `json.RawMessage` is used when a value can have multiple types. Helpers are
  generated for `allOf`, `anyOf`, `oneOf`, `then`, `else` and `dependantSchemas`
  which are references.

## Contributing

Report bugs and send patches to the [mailing list]. Discuss in [#emersion] on
Libera Chat.

## License

MIT

[JSON schema]: https://json-schema.org/
[mailing list]: https://lists.sr.ht/~emersion/public-inbox
[#emersion]: ircs://irc.libera.chat/#emersion

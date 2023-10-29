package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/dave/jennifer/jen"

	"git.sr.ht/~emersion/go-jsonschema"
)

func formatId(s string) string {
	fields := strings.FieldsFunc(s, func(c rune) bool {
		return !unicode.IsLetter(c) && !unicode.IsNumber(c)
	})
	for i, v := range fields {
		fields[i] = strings.Title(v)
	}
	return strings.Join(fields, "")
}

func refName(ref string) string {
	prefix := "#/$defs/"
	if !strings.HasPrefix(ref, prefix) {
		return ""
	}
	return strings.TrimPrefix(ref, prefix)
}

func resolveRef(def *jsonschema.Schema, root *jsonschema.Schema) *jsonschema.Schema {
	if def.Ref == "" {
		return def
	}

	name := refName(def.Ref)
	if name == "" {
		log.Fatalf("unsupported $ref %q", def.Ref)
	}

	result, ok := root.Defs[name]
	if !ok {
		log.Fatalf("invalid $ref %q", def.Ref)
	}
	return &result
}

func schemaType(schema *jsonschema.Schema) jsonschema.Type {
	switch {
	case len(schema.Type) == 1:
		return schema.Type[0]
	case len(schema.Type) > 0:
		return ""
	}

	var v interface{}
	if schema.Const != nil {
		v = schema.Const
	} else if len(schema.Enum) > 0 {
		v = schema.Enum[0]
	}

	switch v.(type) {
	case bool:
		return jsonschema.TypeBoolean
	case map[string]interface{}:
		return jsonschema.TypeObject
	case []interface{}:
		return jsonschema.TypeArray
	case float64:
		return jsonschema.TypeNumber
	case string:
		return jsonschema.TypeString
	default:
		return ""
	}
}

func isRequired(schema *jsonschema.Schema, propName string) bool {
	for _, name := range schema.Required {
		if name == propName {
			return true
		}
	}
	return false
}

func generateStruct(schema *jsonschema.Schema, root *jsonschema.Schema) jen.Code {
	var names []string
	for name := range schema.Properties {
		names = append(names, name)
	}
	sort.Strings(names)

	var fields []jen.Code
	for _, name := range names {
		prop := schema.Properties[name]
		id := formatId(name)
		required := isRequired(schema, name)
		t := generateSchemaType(&prop, root, required)
		jsonTag := name
		if !required {
			jsonTag += ",omitempty"
		}
		tags := map[string]string{"json": jsonTag}
		fields = append(fields, jen.Id(id).Add(t).Tag(tags))
	}
	return jen.Struct(fields...)
}

func singlePatternProp(schema *jsonschema.Schema) *jsonschema.Schema {
	if len(schema.PatternProperties) != 1 {
		return nil
	}
	for _, prop := range schema.PatternProperties {
		return &prop
	}
	return nil
}

func noAdditionalProps(schema *jsonschema.Schema) bool {
	return schema.AdditionalProperties == nil || schema.AdditionalProperties.IsFalse()
}

// unwrapNullableSchema unwraps a schema in the form:
//
//	{
//		"oneOf": {
//			{ "type": "null" },
//			<sub-schema>
//		}
//	}
func unwrapNullableSchema(schema *jsonschema.Schema) (*jsonschema.Schema, bool) {
	for _, choices := range [][]jsonschema.Schema{schema.AnyOf, schema.OneOf} {
		if len(choices) != 2 {
			continue
		}

		nullIndex := -1
		for i, choice := range choices {
			if len(choice.Type) == 1 && choice.Type[0] == jsonschema.TypeNull {
				nullIndex = i
				break
			}
		}
		if nullIndex < 0 {
			continue
		}

		otherIndex := (nullIndex + 1) % 2
		return &choices[otherIndex], true
	}
	return nil, false
}

func generateSchemaType(schema *jsonschema.Schema, root *jsonschema.Schema, required bool) jen.Code {
	if schema == nil {
		schema = &jsonschema.Schema{}
	}

	refName := refName(schema.Ref)
	if refName != "" {
		schema = resolveRef(schema, root)
		t := jen.Id(formatId(refName))
		if !required && schemaType(schema) == jsonschema.TypeObject && noAdditionalProps(schema) && len(schema.PatternProperties) == 0 {
			t = jen.Op("*").Add(t)
		}
		return t
	}

	if subschema, ok := unwrapNullableSchema(schema); ok {
		return jen.Op("*").Add(generateSchemaType(subschema, root, true))
	}

	switch schemaType(schema) {
	case jsonschema.TypeNull:
		return jen.Struct()
	case jsonschema.TypeBoolean:
		return jen.Bool()
	case jsonschema.TypeArray:
		return jen.Index().Add(generateSchemaType(schema.Items, root, required))
	case jsonschema.TypeNumber:
		return jen.Qual("encoding/json", "Number")
	case jsonschema.TypeString:
		return jen.String()
	case jsonschema.TypeInteger:
		return jen.Int64()
	case jsonschema.TypeObject:
		noAdditionalProps := noAdditionalProps(schema)
		if noAdditionalProps && len(schema.PatternProperties) == 0 {
			t := generateStruct(schema, root)
			if !required {
				t = jen.Op("*").Add(t)
			}
			return t
		} else if patternProp := singlePatternProp(schema); noAdditionalProps && patternProp != nil {
			return jen.Map(jen.String()).Add(generateSchemaType(patternProp, root, true))
		} else {
			return jen.Map(jen.String()).Add(generateSchemaType(schema.AdditionalProperties, root, true))
		}
	default:
		return jen.Qual("encoding/json", "RawMessage")
	}
}

func generateDef(schema *jsonschema.Schema, root *jsonschema.Schema, f *jen.File, name string) {
	id := formatId(name)

	if schema.Ref == "" && schemaType(schema) == "" {
		f.Type().Id(id).Struct(
			jen.Qual("encoding/json", "RawMessage"),
		).Line()

		var children []jsonschema.Schema
		for _, child := range schema.AllOf {
			children = append(children, child)
		}
		for _, child := range schema.AnyOf {
			children = append(children, child)
		}
		for _, child := range schema.OneOf {
			children = append(children, child)
		}
		if schema.Then != nil {
			children = append(children, *schema.Then)
		}
		if schema.Else != nil {
			children = append(children, *schema.Else)
		}
		for _, child := range schema.DependentSchemas {
			children = append(children, child)
		}

		for _, child := range children {
			refName := refName(child.Ref)
			if refName == "" {
				continue
			}

			t := generateSchemaType(&child, root, false)

			f.Func().Params(
				jen.Id("v").Id(id),
			).Id(formatId(refName)).Params().Params(
				t,
				jen.Id("error"),
			).Block(
				jen.Var().Id("out").Add(t),
				jen.Id("err").Op(":=").Qual("encoding/json", "Unmarshal").Params(
					jen.Id("v").Op(".").Id("RawMessage"),
					jen.Op("&").Id("out"),
				),
				jen.Return(
					jen.Id("out"),
					jen.Id("err"),
				),
			).Line()
		}
	} else {
		f.Type().Id(id).Add(generateSchemaType(schema, root, true)).Line()
	}
}

func loadSchema(filename string) *jsonschema.Schema {
	f, err := os.Open(filename)
	if err != nil {
		log.Fatalf("failed to open schema file: %v", err)
	}
	defer f.Close()

	var schema jsonschema.Schema
	if err := json.NewDecoder(f).Decode(&schema); err != nil {
		log.Fatalf("failed to load schema JSON: %v", err)
	}

	return &schema
}

const usage = `usage: jsonschemagen -s <schema> -o <output> [options...]

Generate Go types and helpers for the specified JSON schema.

Options:

  -s <schema>    JSON schema filename. Required.
  -o <output>    Output filename for generated Go code. Required.
  -n <package>   Go package name, defaults to the dirname of the output file.
`

func main() {
	var schemaFilename, outputFilename, pkgName string
	flag.StringVar(&schemaFilename, "s", "", "schema filename")
	flag.StringVar(&outputFilename, "o", "", "output filename")
	flag.StringVar(&pkgName, "n", "", "package name")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, usage)
	}
	flag.Parse()

	if schemaFilename == "" || outputFilename == "" || len(flag.Args()) > 0 {
		flag.Usage()
		os.Exit(1)
	}

	if pkgName == "" {
		abs, err := filepath.Abs(outputFilename)
		if err != nil {
			log.Fatalf("failed to get absolute output filename: %v", err)
		}
		pkgName = filepath.Base(filepath.Dir(abs))
	}

	schema := loadSchema(schemaFilename)
	f := jen.NewFile(pkgName)

	if schema.Ref == "" {
		generateDef(schema, schema, f, "root")
	}

	var names []string
	for name := range schema.Defs {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		def := schema.Defs[name]
		generateDef(&def, schema, f, name)
	}

	if err := f.Save(outputFilename); err != nil {
		log.Fatalf("failed to save output file: %v", err)
	}
}

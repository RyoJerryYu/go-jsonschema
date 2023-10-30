package main

import (
	"encoding/json"
	"io"
	"log"
	"sort"
	"strings"
	"unicode"

	"git.sr.ht/~emersion/go-jsonschema"
	"github.com/dave/jennifer/jen"
)

func loadSchema(in io.Reader) *jsonschema.Schema {
	var schema jsonschema.Schema
	if err := json.NewDecoder(in).Decode(&schema); err != nil {
		log.Fatalf("failed to load schema JSON: %v", err)
	}

	return &schema
}

func formatRootStructName(schema *jsonschema.Schema) string {
	if len(schema.Title) > 0 {
		return schema.Title
	}

	return "root"
}

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
	noAdditionalProps := noAdditionalProps(schema)
	noPatternProps := len(schema.PatternProperties) == 0

	if noAdditionalProps && noPatternProps {
		return jen.Struct(fields...) // No additional properties, early return
	}

	additionPropsId := formatId("other-props")
	additionPropsT := jen.Map(jen.String()).Add(jen.Qual("encoding/json", "RawMessage"))
	additionPropsTags := map[string]string{"json": "-"}

	if patternProp := singlePatternProp(schema); noAdditionalProps && patternProp != nil {
		// Only one pattern properties, use the pattern type
		additionPropsT = jen.Map(jen.String()).Add(generateSchemaType(patternProp, root, true))
	} else if schema.AdditionalProperties.IsSchema() && noPatternProps {
		// Only additional properties, use the additional properties type
		prop := schema.AdditionalProperties.Schema
		additionPropsT = jen.Map(jen.String()).Add(generateSchemaType(prop, root, true))
	}
	fields = append(fields,
		jen.Line(),
		jen.Comment("Additional properties, not valided now"),
		jen.Id(additionPropsId).
			Add(additionPropsT).
			Tag(additionPropsTags),
	)
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
	return schema.AdditionalProperties != nil && schema.AdditionalProperties.IsFalse()
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
		t := generateStruct(schema, root)
		if !required {
			t = jen.Op("*").Add(t)
		}
		return t
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
		children = append(children, schema.AllOf...)
		children = append(children, schema.AnyOf...)
		children = append(children, schema.OneOf...)
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

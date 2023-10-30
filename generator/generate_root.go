package generator

import (
	"log"
	"sort"
	"strings"
	"unicode"

	"git.sr.ht/~emersion/go-jsonschema"
	"github.com/dave/jennifer/jen"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// GenerateRoot generates the root struct and all its children
//
//	schema: the inputed root schema
//	f: the result root go AST, which may render to a file, is a return value
func GenerateRoot(schema *jsonschema.Schema, f *jen.File) {
	generator := &Generator{
		root: schema,
		file: f,
	}
	if schema.Ref == "" {
		generator.generateDef(schema, formatRootStructName(schema))
	}

	var names []string
	for name := range schema.Defs {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		def := schema.Defs[name]
		generator.generateDef(&def, name)
	}
}

type Generator struct {
	root *jsonschema.Schema
	file *jen.File
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
		fields[i] = cases.Title(language.Und).String(v)
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

func (g *Generator) resolveRef(def *jsonschema.Schema) *jsonschema.Schema {
	if def.Ref == "" {
		return def
	}

	name := refName(def.Ref)
	if name == "" {
		log.Fatalf("unsupported $ref %q", def.Ref)
	}

	result, ok := g.root.Defs[name]
	if !ok {
		log.Fatalf("invalid $ref %q", def.Ref)
	}
	return &result
}

func (g *Generator) generateStruct(schema *jsonschema.Schema) jen.Code {
	var names []string
	for name := range schema.Properties {
		names = append(names, name)
	}
	sort.Strings(names)

	var fields []jen.Code
	for _, name := range names {
		prop := schema.Properties[name]
		id := formatId(name)
		required := schema.IsRequired(name)
		t := g.generateSchemaType(&prop, required)
		jsonTag := name
		if !required {
			jsonTag += ",omitempty"
		}
		tags := map[string]string{"json": jsonTag}
		fields = append(fields, jen.Id(id).Add(t).Tag(tags))
	}
	noAdditionalProps := schema.NoAdditionalProps()
	noPatternProps := len(schema.PatternProperties) == 0

	if noAdditionalProps && noPatternProps {
		return jen.Struct(fields...) // No additional properties, early return
	}

	additionPropsId := formatId("other-props")
	additionPropsT := jen.Map(jen.String()).Add(jen.Qual("encoding/json", "RawMessage"))
	additionPropsTags := map[string]string{"json": "-"}

	if patternProp := schema.SinglePatternProp(); noAdditionalProps && patternProp != nil {
		// Only one pattern properties, use the pattern type
		additionPropsT = jen.Map(jen.String()).Add(g.generateSchemaType(patternProp, true))
	} else if schema.AdditionalProperties.IsSchema() && noPatternProps {
		// Only additional properties, use the additional properties type
		prop := schema.AdditionalProperties.Schema
		additionPropsT = jen.Map(jen.String()).Add(g.generateSchemaType(prop, true))
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

func (g *Generator) generateSchemaType(schema *jsonschema.Schema, required bool) jen.Code {
	if schema == nil {
		schema = &jsonschema.Schema{}
	}

	refName := refName(schema.Ref)
	if refName != "" {
		schema = g.resolveRef(schema)
		t := jen.Id(formatId(refName))
		if !required && schema.SchemaType() == jsonschema.TypeObject && schema.NoAdditionalProps() && len(schema.PatternProperties) == 0 {
			t = jen.Op("*").Add(t)
		}
		return t
	}

	if subschema, ok := schema.UnwrapNullableSchema(); ok {
		return jen.Op("*").Add(g.generateSchemaType(subschema, true))
	}

	switch schema.SchemaType() {
	case jsonschema.TypeNull:
		return jen.Struct()
	case jsonschema.TypeBoolean:
		return jen.Bool()
	case jsonschema.TypeArray:
		return jen.Index().Add(g.generateSchemaType(schema.Items, required))
	case jsonschema.TypeNumber:
		return jen.Qual("encoding/json", "Number")
	case jsonschema.TypeString:
		return jen.String()
	case jsonschema.TypeInteger:
		return jen.Int64()
	case jsonschema.TypeObject:
		t := g.generateStruct(schema)
		if !required {
			t = jen.Op("*").Add(t)
		}
		return t
	default:
		return jen.Qual("encoding/json", "RawMessage")
	}
}

func (g *Generator) generateDef(schema *jsonschema.Schema, name string) {
	id := formatId(name)

	if schema.Ref == "" && schema.SchemaType() == "" {
		g.file.Type().Id(id).Struct(
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

			t := g.generateSchemaType(&child, false)

			g.file.Func().Params(
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
		g.file.Type().Id(id).Add(g.generateSchemaType(schema, true)).Line()
	}
}

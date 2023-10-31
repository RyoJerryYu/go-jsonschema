package generator

import (
	"sort"
	"strings"

	"git.sr.ht/~emersion/go-jsonschema"
	"github.com/dave/jennifer/jen"
	"github.com/go-errors/errors"
)

// GenerateRoot generates the root struct and all its children
//
//	schema: the inputed root schema
//	f: the result root go AST, which may render to a file, is a return value
func GenerateRoot(opts *GeneratorOptions, f *jen.File, schemas ...*jsonschema.Schema) error {

	generator, err := NewGenerator(opts, f, schemas...)
	if err != nil {
		return errors.New(err)
	}

	for _, schema := range schemas {
		if schema.Ref == "" {
			generator.generateDef(schema)
		}

		var names []string
		for name := range schema.Defs {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			def := schema.Defs[name]
			generator.generateDef(def)
		}
	}
	return nil
}

type GeneratorOptions struct {
	// will not generate additional properties and pattern properties by default
	WithAdditionalProperties bool
}

type Generator struct {
	opts     *GeneratorOptions
	schemas  []*jsonschema.Schema
	resolver *RefResolver
	file     *jen.File
}

func NewGenerator(opts *GeneratorOptions, f *jen.File, schemas ...*jsonschema.Schema) (*Generator, error) {
	resolver, err := NewRefResolver(schemas)
	if err != nil {
		return nil, errors.New(err)
	}
	generator := &Generator{
		opts:     opts,
		schemas:  schemas,
		resolver: resolver,
		file:     f,
	}
	return generator, nil
}

func refName(ref string) string {
	prefix := "#/$defs/"
	if !strings.HasPrefix(ref, prefix) {
		return ref
	}
	return strings.TrimPrefix(ref, prefix)
}

func (g *Generator) resolveRef(def *jsonschema.Schema) (*jsonschema.Schema, error) {
	return g.resolver.GetSchemaByReference(def)
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
		id := toGolangName(name)
		required := schema.IsRequired(name)
		t := g.generateSchemaType(prop, required)
		jsonTag := name
		if !required {
			jsonTag += ",omitempty"
		}
		tags := map[string]string{"json": jsonTag}
		fields = append(fields, jen.Id(id).Add(t).Tag(tags))
	}
	noAdditionalProps := schema.NoAdditionalProps()
	noPatternProps := len(schema.PatternProperties) == 0

	if !g.opts.WithAdditionalProperties || (noAdditionalProps && noPatternProps) {
		return jen.Struct(fields...) // No additional properties, early return
	}

	additionPropsId := toGolangName("other-props")
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
		var err error
		schema, err = g.resolveRef(schema)
		if err != nil {
			return jen.Qual("encoding/json", "RawMessage")
		}
		t := jen.Id(SchemaTypeName(schema))
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

func (g *Generator) generateDef(schema *jsonschema.Schema) {
	id := SchemaTypeName(schema)

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
			).Id(toGolangName(refName)).Params().Params(
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

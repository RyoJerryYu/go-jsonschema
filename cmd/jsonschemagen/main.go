package main

import (
	"encoding/json"
	"log"
	"os"
	"strings"

	"github.com/dave/jennifer/jen"

	"git.sr.ht/~emersion/go-jsonschema"
)

func formatId(s string) string {
	s = strings.Title(s)
	// TODO: improve robustness
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, "_", "")
	return s
}

func resolveRef(def *jsonschema.Schema, root *jsonschema.Schema) *jsonschema.Schema {
	if def.Ref == "" {
		return def
	}

	prefix := "#/$defs/"
	if !strings.HasPrefix(def.Ref, prefix) {
		log.Fatalf("unsupported $ref %q", def.Ref)
	}
	name := strings.TrimPrefix(def.Ref, prefix)

	result, ok := root.Defs[name]
	if !ok {
		log.Fatalf("invalid $ref %q", def.Ref)
	}
	return &result
}

func schemaType(schema *jsonschema.Schema) jsonschema.Type {
	if schema.Type != "" {
		return schema.Type
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

func generateSchemaType(schema *jsonschema.Schema, root *jsonschema.Schema) jen.Code {
	if schema == nil {
		return jen.Interface()
	}

	schema = resolveRef(schema, root)
	switch schemaType(schema) {
	case jsonschema.TypeNull:
		return jen.Struct()
	case jsonschema.TypeBoolean:
		return jen.Bool()
	case jsonschema.TypeArray:
		return jen.Index().Add(generateSchemaType(schema.Items, root))
	case jsonschema.TypeNumber:
		return jen.Float64()
	case jsonschema.TypeString:
		return jen.String()
	case jsonschema.TypeInteger:
		return jen.Int64()
	case jsonschema.TypeObject:
		return jen.Map(jen.String()).Add(generateSchemaType(schema.AdditionalProperties, root))
	default:
		return jen.Interface()
	}
}

func generateDef(def *jsonschema.Schema, root *jsonschema.Schema, f *jen.File, name string) {
	if schemaType(def) != jsonschema.TypeObject {
		return
	}
	if def.AdditionalProperties == nil || !def.AdditionalProperties.IsFalse() {
		return
	}
	if len(def.PatternProperties) > 0 {
		return
	}

	var fields []jen.Code
	for name, prop := range def.Properties {
		id := formatId(name)
		t := generateSchemaType(&prop, root)
		tags := map[string]string{"json": name}
		fields = append(fields, jen.Id(id).Add(t).Tag(tags))
	}

	f.Type().Id(formatId(name)).Struct(fields...).Line()
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

func main() {
	if len(os.Args) != 4 {
		log.Fatalf("usage: jsonschemagen <schema> <output> <package>")
	}

	inputFilename := os.Args[1]
	outputFilename := os.Args[2]
	pkgName := os.Args[3]

	schema := loadSchema(inputFilename)
	f := jen.NewFile(pkgName)

	generateDef(schema, schema, f, "root")
	for k, def := range schema.Defs {
		generateDef(&def, schema, f, k)
	}

	if err := f.Save(outputFilename); err != nil {
		log.Fatalf("failed to save output file: %v", err)
	}
}

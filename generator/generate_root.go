package generator

import (
	"sort"

	"git.sr.ht/~emersion/go-jsonschema"
	"github.com/dave/jennifer/jen"
	"github.com/go-errors/errors"
)

// GenerateRoot generates the root struct and all its children
//
//	schema: the inputed root schema
//	f: the result root go AST, which may render to a file, is a return value
func GenerateRoot(f *jen.File, schemas ...*jsonschema.Schema) error {

	generator, err := NewGenerator(f, schemas...)
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
			// TODO check for duplicate names
			generator.generateDef(def)
		}
	}
	return nil
}

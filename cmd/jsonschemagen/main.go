package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"git.sr.ht/~emersion/go-jsonschema/generator"
	"git.sr.ht/~emersion/go-jsonschema/loader"
	"github.com/dave/jennifer/jen"
	"github.com/go-errors/errors"
	"github.com/iancoleman/strcase"
	"github.com/spf13/cobra"
)

type Flags struct {
	SchemaFilename string
	OutputFilename string
	PkgName        string
}

func LoadFlags() *Flags {
	f := &Flags{}

	return f
}

func (f *Flags) formatOutputFilename(filename string) string {
	abs, err := filepath.Abs(filename)
	if err != nil {
		log.Fatalf("failed to get absolute filename: %v", err)
	}

	return strings.Replace(abs, "$", "", -1)
}

func (f *Flags) isStdout() bool {
	return f.OutputFilename == ""
}

func (f *Flags) Format() error {
	if f.OutputFilename == "-" {
		f.OutputFilename = "" // stdout
	}

	if !f.isStdout() {
		f.OutputFilename = f.formatOutputFilename(f.OutputFilename)
	}
	if f.PkgName == "" {
		f.PkgName = filepath.Base(filepath.Dir(f.OutputFilename))
	}

	f.PkgName = strcase.ToSnake(f.PkgName)
	if f.PkgName == "type" {
		f.PkgName = "pkg_type"
	}

	return nil
}

func (f *Flags) PipeOut() (io.WriteCloser, error) {
	if f.isStdout() {
		return os.Stdout, nil
	}

	if err := os.MkdirAll(filepath.Dir(f.OutputFilename), 0755); err != nil {
		return nil, err
	}

	w, err := os.Create(f.OutputFilename)
	if err != nil {
		return nil, err
	}
	return w, nil
}

func main() {
	cmd := cobra.Command{
		Use:   "jsonschemagen [flags] [schema file]...",
		Short: "Generate Go types and helpers for the specified JSON schema.",
		Long: `Generate Go types and helpers for the specified JSON schema.
If no schema file is specified or specified to "-", read from stdin.

Example:
$ find schema -name '*.json' | xargs jsonschemagen --rootdir=$PWD -n out > out/generated.go
`,
	}
	flags := Flags{}
	cmd.Flags().StringVarP(&flags.SchemaFilename, "schema", "s", "", `The schema filename, deprecated.
recommended to use positional argument.
"-" for stdin.`)
	cmd.Flags().StringVarP(&flags.OutputFilename, "output", "o", "", `The output filename.
If not provided or specified to "-", output to stdout.`)
	cmd.Flags().StringVarP(&flags.PkgName, "packagename", "n", "", "package name")

	loaderOpts := loader.ParseOptions{}
	cmd.Flags().StringVar(&loaderOpts.BaseURI, "baseuri", "", "base URI")
	cmd.Flags().StringVar(&loaderOpts.RootDir, "rootdir", "", "root directory")

	generatorOpts := generator.GeneratorOptions{}
	cmd.Flags().BoolVar(&generatorOpts.WithAdditionalProperties, "with-additional-properties", false, "Generate additional properties and pattern properties")
	cmd.Flags().StringSliceVarP(&generatorOpts.UpperPropertyNames, "upper-property-names", "u", nil, `Apply full upper case to the property names.
e.g. given "id", "Id" or "ID" as flags, when a type or field name 
parsed as "Id", would be converted as "ID"`)

	cmd.Run = func(cmd *cobra.Command, args []string) {
		err := flags.Format()
		checkError(err)

		filePaths := args[:]
		if flags.SchemaFilename != "" {
			filePaths = append(filePaths, flags.SchemaFilename)
		}

		schemas, err := loader.New(&loaderOpts).LoadAll(filePaths)
		checkError(err)
		f := jen.NewFile(flags.PkgName)

		err = generator.GenerateRoot(&generatorOpts, f, schemas...)
		checkError(err)

		pipeOut, err := flags.PipeOut()
		checkError(err)
		defer pipeOut.Close()

		if err := f.Render(pipeOut); err != nil {
			checkError(err)
		}
	}

	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}

}

func checkError(err error) {
	if err != nil {
		if e, ok := err.(*errors.Error); ok {
			fmt.Fprintln(os.Stderr, string(e.Stack()))
		} else {
			log.Fatalf("error: %v", err)
		}

		os.Exit(1)
	}
}

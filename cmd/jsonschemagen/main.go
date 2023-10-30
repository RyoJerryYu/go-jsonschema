package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dave/jennifer/jen"
)

const usage = `usage: jsonschemagen -s <schema> -o <output> [options...]

Generate Go types and helpers for the specified JSON schema.

Options:

  -s <schema>    JSON schema filename. Required.
  -o <output>    Output filename for generated Go code. Required.
  -n <package>   Go package name, defaults to the dirname of the output file.
`

type Flags struct {
	SchemaFilename string
	OutputFilename string
	PkgName        string
	// Args           []string
}

func LoadFlags() *Flags {
	f := &Flags{}
	flag.StringVar(&f.SchemaFilename, "s", "", "schema filename")
	flag.StringVar(&f.OutputFilename, "o", "", "output filename")
	flag.StringVar(&f.PkgName, "n", "", "package name")

	flag.Usage = func() {
		fmt.Fprint(os.Stderr, usage)
	}

	flag.Parse()
	// f.Args = flag.Args()

	if err := f.Format(); err != nil {
		fmt.Fprintf(os.Stderr, "invalid arguments: %v", err)
		flag.Usage()
		os.Exit(1)
	}

	return f
}

func (f *Flags) formatOutputFilename(filename string) string {
	abs, err := filepath.Abs(filename)
	if err != nil {
		log.Fatalf("failed to get absolute filename: %v", err)
	}

	return strings.Replace(abs, "$", "", -1)
}

func (f *Flags) isStdin() bool {
	return f.SchemaFilename == ""
}
func (f *Flags) isStdout() bool {
	return f.OutputFilename == ""
}

func (f *Flags) Format() error {
	if f.SchemaFilename == "-" {
		f.SchemaFilename = "" // stdin
	}
	if f.OutputFilename == "-" {
		f.OutputFilename = "" // stdout
	}

	if !f.isStdout() {
		f.OutputFilename = f.formatOutputFilename(f.OutputFilename)
	}
	if f.PkgName == "" {
		f.PkgName = filepath.Base(filepath.Dir(f.OutputFilename))
	}

	return nil
}

func (f *Flags) Input() (io.ReadCloser, error) {
	if f.isStdin() {
		return os.Stdin, nil
	}

	r, err := os.Open(f.SchemaFilename)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (f *Flags) Output() (io.WriteCloser, error) {
	if f.isStdout() {
		return os.Stdout, nil
	}

	w, err := os.Create(f.OutputFilename)
	if err != nil {
		return nil, err
	}
	return w, nil
}

func main() {
	flags := LoadFlags()

	input, err := flags.Input()
	if err != nil {
		log.Fatalf("failed to open schema file: %v", err)
	}
	defer input.Close()

	schema := loadSchema(input)
	f := jen.NewFile(flags.PkgName)

	if schema.Ref == "" {
		generateDef(schema, schema, f, formatRootStructName(schema))
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

	output, err := flags.Output()
	if err != nil {
		log.Fatalf("failed to open output file: %v", err)
	}
	defer output.Close()

	if err := f.Render(output); err != nil {
		log.Fatalf("failed to render output file: %v", err)
	}
}

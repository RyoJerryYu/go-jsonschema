package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"git.sr.ht/~emersion/go-jsonschema"
	"git.sr.ht/~emersion/go-jsonschema/generator"
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

func (f *Flags) PipeIn() (io.ReadCloser, error) {
	if f.isStdin() {
		return os.Stdin, nil
	}

	r, err := os.Open(f.SchemaFilename)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (f *Flags) PipeOut() (io.WriteCloser, error) {
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

	pipeIn, err := flags.PipeIn()
	if err != nil {
		log.Fatalf("failed to open schema file: %v", err)
	}
	defer pipeIn.Close()

	schema := jsonschema.LoadSchema(pipeIn)
	f := jen.NewFile(flags.PkgName)

	generator.GenerateRoot(schema, f)

	pipeOut, err := flags.PipeOut()
	if err != nil {
		log.Fatalf("failed to open output file: %v", err)
	}
	defer pipeOut.Close()

	if err := f.Render(pipeOut); err != nil {
		log.Fatalf("failed to render output file: %v", err)
	}
}

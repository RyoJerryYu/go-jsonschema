package loader

import (
	"io"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/RyoJerryYu/go-jsonschema"
	"github.com/go-errors/errors"
)

func abs(name string) (string, error) {
	if path.IsAbs(name) {
		return name, nil
	}
	wd, err := os.Getwd()
	return path.Join(wd, name), err
}

type ParseOptions struct {
	RootDir string
	BaseURI string
}

type Loader struct {
	opts *ParseOptions
}

func New(opts *ParseOptions) *Loader {
	return &Loader{opts: opts}
}

func (l *Loader) ParseFileURI(file string) (*url.URL, error) {
	abPath, err := abs(file)
	if err != nil {
		return nil, err
	}
	abPath = strings.TrimPrefix(abPath, l.opts.RootDir)
	if !strings.HasPrefix(abPath, "/") {
		abPath = "/" + abPath
	}

	baseUri := &url.URL{
		Scheme: "file",
	}
	if l.opts.BaseURI != "" {
		baseUri, err = url.Parse(l.opts.BaseURI)
		if err != nil {
			return nil, err
		}
	}

	baseUri.Path = abPath

	return baseUri, nil
}

func (l *Loader) LoadAll(filePaths []string) ([]*jsonschema.Schema, error) {
	if len(filePaths) == 0 {
		schema, err := l.LoadStdin()
		if err != nil {
			return nil, errors.New(err)
		}

		return []*jsonschema.Schema{schema}, nil
	}

	schemas := make([]*jsonschema.Schema, len(filePaths))
	for i, filePath := range filePaths {
		schema, err := l.LoadFile(filePath)
		if err != nil {
			return nil, errors.New(err)
		}

		schemas[i] = schema
	}

	return schemas, nil
}

// LoadFile loads a JSON schema from filePath
// if rootDir is not empty, schema uri is relative to rootDir
// if baseUri is not empty, schema uri will be resolved against baseUri
func (l *Loader) LoadFile(filePath string) (*jsonschema.Schema, error) {
	input, err := os.Open(filePath)
	if err != nil {
		return nil, errors.New(err)
	}
	defer input.Close()

	fileURI, err := l.ParseFileURI(filePath)
	if err != nil {
		return nil, errors.New(err)
	}

	return l.loadInput(input, fileURI)
}

func (l *Loader) LoadStdin() (*jsonschema.Schema, error) {
	return l.loadInput(os.Stdin, &url.URL{})
}

func (l *Loader) loadInput(input io.Reader, fileUri *url.URL) (*jsonschema.Schema, error) {
	schema := jsonschema.LoadSchema(input, fileUri)
	return schema, nil
}

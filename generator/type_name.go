package generator

import (
	"bytes"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/RyoJerryYu/go-jsonschema"
)

func (g *Generator) SchemaTypeName(schema *jsonschema.Schema) string {
	name := getIdentifier(schema)
	return g.toGolangName(name)
}

// toGolangName strips invalid characters out of golang struct or field names.
func (g *Generator) toGolangName(s string) string {
	buf := bytes.NewBuffer([]byte{})
	fields := strings.FieldsFunc(s, func(c rune) bool {
		return !unicode.IsLetter(c) && !unicode.IsNumber(c)
	})

	for i, v := range fields {
		if v == "" {
			continue
		}
		if i == 0 && strings.IndexAny(v, "0123456789") == 0 {
			// Go types are not allowed to start with a number, lets prefix with an underscore.
			buf.WriteRune('_')
		}

		buf.WriteString(strings.ToUpper(v[:1]) + v[1:])
	}
	result := buf.String()
	for _, keyword := range g.opts.UpperPropertyNames {
		if upper := strings.ToUpper(result); upper == strings.ToUpper(keyword) {
			result = upper
		}
	}

	return result
}

type patternMatcher struct {
	patterns         *regexp.Regexp
	modify           func(string) string
	shouldIncludeDir bool
}

var (
	// index or is a path parameter
	shouldIgnorePattern = patternMatcher{
		patterns: regexp.MustCompile(`(^index$|^\$)`),
		modify: func(name string) string {
			return ""
		},
		shouldIncludeDir: true,
	}
	// start with number or underscores
	invalidLeading = patternMatcher{
		patterns: regexp.MustCompile(`^[0-9_\-\.]+`),
		modify: func(name string) string {
			name = strings.TrimPrefix(name, "_")
			name = strings.TrimPrefix(name, "-")
			name = strings.TrimPrefix(name, ".")
			return name
		},
		shouldIncludeDir: true,
	}
	// contains dash
	includeDash = patternMatcher{
		patterns: regexp.MustCompile(`-`),
		modify: func(name string) string {
			return strings.ReplaceAll(name, "-", "_")
		},
		shouldIncludeDir: false,
	}

	matchers = []*patternMatcher{&invalidLeading, &includeDash, &shouldIgnorePattern}
)

func getIdentifier(schema *jsonschema.Schema) string {
	if len(schema.Title) > 0 {
		return schema.Title
	}

	uri := mustPathFromSchema(schema)
	name := strings.TrimSuffix(filepath.Base(uri), ".json") // filename without extension
	dir := filepath.Dir(uri)

MATCH_LOOP:
	for {
		for _, matcher := range matchers {
			if matcher.patterns.MatchString(name) {
				name = matcher.modify(name)
				if !matcher.shouldIncludeDir {
					continue MATCH_LOOP
				}

				base := baseOrRoot(dir)
				name = base + "_" + name
				name = strings.TrimSuffix(name, "_") // when name was empty

				if base == "root" {
					return name
				}
				dir = filepath.Dir(dir)
				continue MATCH_LOOP
			}
		}
		return name
	}
}

func mustPathFromSchema(schema *jsonschema.Schema) string {
	u, err := url.Parse(schema.ID)
	if err != nil {
		panic(err)
	}

	return u.Path
}

func baseOrRoot(path string) string {
	if path == "." || path == "" {
		return "root"
	}
	return filepath.Base(path)
}

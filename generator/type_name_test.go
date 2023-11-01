package generator

import (
	"testing"

	"git.sr.ht/~emersion/go-jsonschema"
)

func TestThatJavascriptKeyNamesCanBeConvertedToValidGoNames(t *testing.T) {
	tests := []struct {
		description string
		input       string
		expected    string
	}{
		{
			description: "Camel case is converted to pascal case.",
			input:       "camelCase",
			expected:    "CamelCase",
		},
		{
			description: "Spaces are stripped.",
			input:       "Contains space",
			expected:    "ContainsSpace",
		},
		{
			description: "Hyphens are stripped.",
			input:       "key-name",
			expected:    "KeyName",
		},
		{
			description: "Underscores are stripped.",
			input:       "key_name",
			expected:    "KeyName",
		},
		{
			description: "Periods are stripped.",
			input:       "a.b.c",
			expected:    "ABC",
		},
		{
			description: "Colons are stripped.",
			input:       "a:b",
			expected:    "AB",
		},
		{
			description: "GT and LT are stripped.",
			input:       "a<b>",
			expected:    "AB",
		},
		{
			description: "Not allowed to start with a number.",
			input:       "123ABC",
			expected:    "_123ABC",
		},
	}

	for _, test := range tests {
		g := &Generator{}
		actual := g.toGolangName(test.input)

		if test.expected != actual {
			t.Errorf("For test '%s', for input '%s' expected '%s' but got '%s'.", test.description, test.input, test.expected, actual)
		}

		// actual = formatId(test.input)
		// if test.expected != actual {
		// 	t.Errorf("For test '%s', for input '%s' expected '%s' but got '%s'.", test.description, test.input, test.expected, actual)
		// }
	}
}

func TestFormatRootStructName(t *testing.T) {
	cases := []struct {
		name     string
		schemaID string
		title    string
		expected string
	}{
		{
			name:     "empty",
			schemaID: "",
			title:    "",
			expected: "root",
		},
		{
			name:     "title",
			schemaID: "",
			title:    "title",
			expected: "title",
		},
		{
			name:     "id",
			schemaID: "id",
			title:    "",
			expected: "id",
		},
		{
			name:     "id with extension",
			schemaID: "id.json",
			title:    "",
			expected: "id",
		},
		{
			name:     "id with directory",
			schemaID: "dir/id",
			title:    "",
			expected: "id",
		},
		{
			name:     "index with directory",
			schemaID: "dir/index.json",
			title:    "",
			expected: "dir",
		},
		{
			name:     "index with invalid directory",
			schemaID: "-1/index.json",
			title:    "",
			expected: "root_1",
		},
		{
			name:     "contain dash",
			schemaID: "dir/contain-dash",
			title:    "",
			expected: "contain_dash",
		},
		{
			name:     "contain underscore",
			schemaID: "dir/$id/index.json",
			title:    "",
			expected: "dir",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			schema := &jsonschema.Schema{
				ID:    c.schemaID,
				Title: c.title,
			}
			actual := getIdentifier(schema)
			if actual != c.expected {
				t.Errorf("expected %s, got %s", c.expected, actual)
			}
		})
	}
}

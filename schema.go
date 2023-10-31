package jsonschema

import (
	"encoding/json"
	"io"
	"log"
	"net/url"
)

type Type string

const (
	TypeNull    Type = "null"
	TypeBoolean Type = "boolean"
	TypeObject  Type = "object"
	TypeArray   Type = "array"
	TypeNumber  Type = "number"
	TypeString  Type = "string"
	TypeInteger Type = "integer"
)

type TypeSet []Type

func (ts *TypeSet) UnmarshalJSON(b []byte) error {
	if b[0] == '[' {
		type rawTypeSet TypeSet
		out := (*rawTypeSet)(ts)
		return json.Unmarshal(b, out)
	} else {
		var t Type
		err := json.Unmarshal(b, &t)
		if err != nil {
			*ts = nil
		} else {
			*ts = []Type{t}
		}
		return err
	}
}

type Schema struct {
	// Core
	Schema     string             `json:"$schema"`
	Vocabulary map[string]bool    `json:"$vocabulary"`
	ID         string             `json:"$id"`
	Ref        string             `json:"$ref"`
	DynamicRef string             `json:"$dynamicRef"`
	Defs       map[string]*Schema `json:"$defs"`
	Comment    string             `json:"$comment"`

	// Applying subschemas with logic
	AllOf []Schema `json:"allOf"`
	AnyOf []Schema `json:"anyOf"`
	OneOf []Schema `json:"oneOf"`
	Not   []Schema `json:"not"`

	// Applying subschemas conditionally
	If               *Schema           `json:"if"`
	Then             *Schema           `json:"then"`
	Else             *Schema           `json:"else"`
	DependentSchemas map[string]Schema `json:"dependentSchemas"`

	// Applying subschemas to arrays
	PrefixItems []Schema `json:"prefixItems"`
	Items       *Schema  `json:"items"`
	Contains    *Schema  `json:"contains"`

	// Applying subschemas to objects
	Properties           map[string]*Schema    `json:"properties"`
	PatternProperties    map[string]*Schema    `json:"patternProperties"`
	AdditionalProperties *AdditionalProperties `json:"additionalProperties"`
	PropertyNames        *Schema               `json:"propertyNames"`

	// Validation
	Type  TypeSet       `json:"type"`
	Enum  []interface{} `json:"enum"`
	Const interface{}   `json:"const"`

	// Validation for numbers
	MultipleOf       json.Number `json:"multipleOf"`
	Maximum          json.Number `json:"maximum"`
	ExclusiveMaximum json.Number `json:"exclusiveMaximum"`
	Minimum          json.Number `json:"minimum"`
	ExclusiveMinimum json.Number `json:"exclusiveMinimum"`

	// Validation for strings
	MaxLength int    `json:"maxLength"`
	MinLength int    `json:"minLength"`
	Pattern   string `json:"pattern"`

	// Validation for arrays
	MaxItems    int  `json:"maxItems"`
	MinItems    int  `json:"minItems"`
	UniqueItems bool `json:"uniqueItems"`
	MaxContains int  `json:"maxContains"`
	MinContains int  `json:"minContains"`

	// Validation for objects
	MaxProperties     int                 `json:"maxProperties"`
	MinProperties     int                 `json:"minProperties"`
	Required          []string            `json:"required"`
	DependentRequired map[string][]string `json:"dependentRequired"`

	// Basic metadata annotations
	Title       string        `json:"title"`
	Description string        `json:"description"`
	Default     interface{}   `json:"default"`
	Deprecated  bool          `json:"deprecated"`
	ReadOnly    bool          `json:"readOnly"`
	WriteOnly   bool          `json:"writeOnly"`
	Examples    []interface{} `json:"examples"`
}

func (schema *Schema) UnmarshalJSON(b []byte) error {
	type rawSchema Schema
	var out rawSchema
	if err := json.Unmarshal(b, &out); err != nil {
		return err
	}
	*schema = Schema(out)
	return nil
}

// LoadSchema loads a schema from a JSON file.
// if input schema does not have $id, it will be set to the file URI.
func LoadSchema(in io.Reader, fileUri *url.URL) *Schema {
	var schema Schema
	if err := json.NewDecoder(in).Decode(&schema); err != nil {
		log.Fatalf("failed to load schema JSON: %v", err)
	}

	if schema.ID == "" {
		schema.ID = fileUri.String()
	}

	return &schema
}

func (schema *Schema) SchemaType() Type {
	switch {
	case len(schema.Type) == 1:
		return schema.Type[0]
	case len(schema.Type) > 0:
		return ""
	}

	var v interface{}
	if schema.Const != nil {
		v = schema.Const
	} else if len(schema.Enum) > 0 {
		v = schema.Enum[0]
	}

	switch v.(type) {
	case bool:
		return TypeBoolean
	case map[string]interface{}:
		return TypeObject
	case []interface{}:
		return TypeArray
	case float64:
		return TypeNumber
	case string:
		return TypeString
	default:
		return ""
	}
}

func (schema *Schema) IsRequired(propName string) bool {
	for _, name := range schema.Required {
		if name == propName {
			return true
		}
	}
	return false
}

func (schema *Schema) SinglePatternProp() *Schema {
	if len(schema.PatternProperties) != 1 {
		return nil
	}
	for _, prop := range schema.PatternProperties {
		return prop
	}
	return nil
}

func (schema *Schema) NoAdditionalProps() bool {
	return schema.AdditionalProperties != nil && schema.AdditionalProperties.IsFalse()
}

// UnwrapNullableSchema unwraps a schema in the form:
//
//	{
//		"oneOf": {
//			{ "type": "null" },
//			<sub-schema>
//		}
//	}
func (schema *Schema) UnwrapNullableSchema() (*Schema, bool) {
	for _, choices := range [][]Schema{schema.AnyOf, schema.OneOf} {
		if len(choices) != 2 {
			continue
		}

		nullIndex := -1
		for i, choice := range choices {
			if len(choice.Type) == 1 && choice.Type[0] == TypeNull {
				nullIndex = i
				break
			}
		}
		if nullIndex < 0 {
			continue
		}

		otherIndex := (nullIndex + 1) % 2
		return &choices[otherIndex], true
	}

	if len(schema.Type) != 2 {
		return nil, false
	}

	nullIndex := -1
	for i, t := range schema.Type {
		if t == TypeNull {
			nullIndex = i
			break
		}
	}

	if nullIndex < 0 {
		return nil, false
	}

	otherIndex := (nullIndex + 1) % 2
	otherType := schema.Type[otherIndex]
	another := *schema
	another.Type = TypeSet{otherType}
	return &another, true
}

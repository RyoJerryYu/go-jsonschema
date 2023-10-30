package jsonschema

import (
	"encoding/json"
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
	Schema     string            `json:"$schema"`
	Vocabulary map[string]bool   `json:"$vocabulary"`
	ID         string            `json:"$id"`
	Ref        string            `json:"$ref"`
	DynamicRef string            `json:"$dynamicRef"`
	Defs       map[string]Schema `json:"$defs"`
	Comment    string            `json:"$comment"`

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
	Properties           map[string]Schema     `json:"properties"`
	PatternProperties    map[string]Schema     `json:"patternProperties"`
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

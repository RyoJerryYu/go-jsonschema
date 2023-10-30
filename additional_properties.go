package jsonschema

import (
	"bytes"
	"encoding/json"
)

type AdditionalProperties struct {
	*Schema
	additionalPropertiesBool *bool
}

func AdditionalPropertiesBool(b bool) AdditionalProperties {
	return AdditionalProperties{additionalPropertiesBool: &b}
}

func (ap *AdditionalProperties) UnmarshalJSON(b []byte) error {
	if bytes.Equal(b, []byte("true")) {
		*ap = AdditionalPropertiesBool(true)
	} else if bytes.Equal(b, []byte("false")) {
		*ap = AdditionalPropertiesBool(false)
	} else {
		return json.Unmarshal(b, ap.Schema)
	}

	return nil
}

func (ap *AdditionalProperties) MarshalJSON() ([]byte, error) {
	if ap == nil {
		return nil, nil
	}

	if ap.additionalPropertiesBool != nil {
		return json.Marshal(*ap.additionalPropertiesBool)
	}

	return json.Marshal(ap.Schema)
}

func (ap *AdditionalProperties) IsFalse() bool {
	return ap != nil &&
		ap.additionalPropertiesBool != nil &&
		!*ap.additionalPropertiesBool
}

func (ap *AdditionalProperties) IsSchema() bool {
	return ap != nil &&
		ap.Schema != nil
}

func (ap *AdditionalProperties) HaveAdditionalProperties() bool {
	return ap != nil &&
		(ap.Schema != nil || !ap.IsFalse())
}

func (ap *AdditionalProperties) NoAdditionalProperties() bool {
	return ap != nil &&
		ap.Schema == nil &&
		ap.IsFalse()
}

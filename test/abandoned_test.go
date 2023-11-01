package test

import (
	"testing"

	abandoned "github.com/RyoJerryYu/go-jsonschema/test/abandoned_gen"
)

func TestAbandoned(t *testing.T) {
	// this just tests the name generation works correctly
	r := abandoned.Abandoned{
		Name:      "jonson",
		Abandoned: &struct{}{},
	}
	// the test is the presence of the Abandoned field
	if r.Abandoned == nil {
		t.Fatal("thats the test")
	}
}

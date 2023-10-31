package generator

import (
	"fmt"
	"net/url"
	"strings"

	"git.sr.ht/~emersion/go-jsonschema"
	"github.com/go-errors/errors"
)

type RefResolver struct {
	pathToSchema map[string]*jsonschema.Schema
}

func NewRefResolver(schemas []*jsonschema.Schema) (*RefResolver, error) {
	r := &RefResolver{pathToSchema: make(map[string]*jsonschema.Schema)}
	for _, schema := range schemas {
		err := r.mapPaths(schema)
		if err != nil {
			return nil, errors.New(err)
		}
	}
	return r, nil
}

func (r *RefResolver) insert(uri string, schema *jsonschema.Schema) error {
	if _, ok := r.pathToSchema[uri]; ok {
		return fmt.Errorf("attempted to add duplicate uri: %s", uri)
	}
	r.pathToSchema[uri] = schema
	return nil
}

func (r *RefResolver) mapPaths(schema *jsonschema.Schema) error {
	rootURI := &url.URL{}
	id := schema.ID
	if id == "" {
		if err := r.insert("#", schema); err != nil {
			return err
		}
	} else {
		var err error
		rootURI, err = url.Parse(id)
		if err != nil {
			return err
		}
		// ensure no fragment.
		rootURI.Fragment = ""
		if err := r.insert(rootURI.String(), schema); err != nil {
			return err
		}
		// add as JSON pointer (?)
		if err := r.insert(rootURI.String()+"#", schema); err != nil {
			return err
		}
	}
	r.updateURIs(schema, *rootURI, false, false)
	return nil
}

// create a map of base URIs
func (r *RefResolver) updateURIs(schema *jsonschema.Schema, baseURI url.URL, checkCurrentID bool, ignoreFragments bool) error {
	// already done for root, and if schema sets a new base URI
	if checkCurrentID && schema.ID != "" {
		id := schema.ID
		newBase, err := url.Parse(id)
		if err != nil {
			return err
		}
		// if it's a JSON fragment and we're coming from part of the tree where the baseURI has changed, we need to
		// ignore the fragment, since it won't be resolvable under the current baseURI.
		if !(strings.HasPrefix(id, "#") && ignoreFragments) {
			// map all the subschema under the new base
			resolved := baseURI.ResolveReference(newBase)
			if err := r.insert(resolved.String(), schema); err != nil {
				return err
			}
			if resolved.Fragment == "" {
				if err := r.insert(resolved.String()+"#", schema); err != nil {
					return err
				}
			}
			if err := r.updateURIs(schema, *resolved, false, false); err != nil {
				return err
			}
			// and continue to map all subschema under the old base (except for fragments)
			ignoreFragments = true
		}
	}
	for k, subSchema := range schema.Defs {
		newBaseURI := baseURI
		newBaseURI.Fragment += "/definitions/" + k
		if err := r.insert(newBaseURI.String(), subSchema); err != nil {
			return err
		}
		r.updateURIs(subSchema, newBaseURI, true, ignoreFragments)
	}
	for k, subSchema := range schema.Properties {
		newBaseURI := baseURI
		newBaseURI.Fragment += "/properties/" + k
		if err := r.insert(newBaseURI.String(), subSchema); err != nil {
			return err
		}
		r.updateURIs(subSchema, newBaseURI, true, ignoreFragments)
	}
	if schema.AdditionalProperties != nil && schema.AdditionalProperties.IsSchema() {
		newBaseURI := baseURI
		newBaseURI.Fragment += "/additionalProperties"
		r.updateURIs(schema.AdditionalProperties.Schema, newBaseURI, true, ignoreFragments)
	}
	if schema.Items != nil {
		newBaseURI := baseURI
		newBaseURI.Fragment += "/items"
		r.updateURIs(schema.Items, newBaseURI, true, ignoreFragments)
	}
	return nil
}

// GetSchemaByReference returns the schema.
func (r *RefResolver) GetSchemaByReference(schema *jsonschema.Schema) (*jsonschema.Schema, error) {
	u, err := url.Parse(schema.ID)
	if err != nil {
		return nil, err
	}
	ref, err := url.Parse(schema.Ref)
	if err != nil {
		return nil, err
	}
	resolvedPath := u.ResolveReference(ref)
	path, ok := r.pathToSchema[resolvedPath.String()]
	if !ok {
		return nil, errors.New("refresolver.GetSchemaByReference: reference not found: " + schema.Ref)
	}
	return path, nil
}

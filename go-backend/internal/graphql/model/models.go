// Package model contains GraphQL model definitions and custom scalars
package model

import (
	"encoding/json"
	"fmt"
	"io"
)

// JSON is a custom scalar for arbitrary JSON data
type JSON map[string]interface{}

// MarshalGQL implements the graphql.Marshaler interface
func (j JSON) MarshalGQL(w io.Writer) {
	if j == nil {
		w.Write([]byte("null"))
		return
	}
	enc := json.NewEncoder(w)
	enc.Encode(j)
}

// UnmarshalGQL implements the graphql.Unmarshaler interface
func (j *JSON) UnmarshalGQL(v interface{}) error {
	switch t := v.(type) {
	case map[string]interface{}:
		*j = JSON(t)
		return nil
	case nil:
		*j = nil
		return nil
	default:
		return fmt.Errorf("cannot unmarshal %T into JSON", v)
	}
}

// MarshalJSON for standard json package
func (j JSON) MarshalJSON() ([]byte, error) {
	if j == nil {
		return []byte("null"), nil
	}
	return json.Marshal(map[string]interface{}(j))
}

// UnmarshalJSON for standard json package
func (j *JSON) UnmarshalJSON(data []byte) error {
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	*j = JSON(m)
	return nil
}

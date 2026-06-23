// Package jsonx is the single seam between bast and its JSON backend.
// Swap the import here to change the JSON library across the entire framework.
package jsonx

import gojson "github.com/goccy/go-json"

func Marshal(v any) ([]byte, error)   { return gojson.Marshal(v) }
func Unmarshal(data []byte, v any) error { return gojson.Unmarshal(data, v) }

func NewEncoder(w interface{ Write([]byte) (int, error) }) *gojson.Encoder {
	return gojson.NewEncoder(w)
}

func NewDecoder(r interface{ Read([]byte) (int, error) }) *gojson.Decoder {
	return gojson.NewDecoder(r)
}
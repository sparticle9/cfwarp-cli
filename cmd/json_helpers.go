package cmd

import "encoding/json"

func printAnyJSON(v any, enc *json.Encoder) error {
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

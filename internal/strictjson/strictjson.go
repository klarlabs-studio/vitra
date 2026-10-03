// Package strictjson decodes untrusted frontend input strictly: unknown
// object fields, mismatched types, and trailing data are rejected.
package strictjson

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

// Decode converts input, a value the IPC bridge decoded from JSON into
// `any`, into T. It is how typed commands receive their input.
func Decode[T any](input any) (T, error) {
	var out T
	raw, err := json.Marshal(input)
	if err != nil {
		return out, err
	}
	err = Unmarshal(raw, &out)
	return out, err
}

// Unmarshal decodes data into v like json.Unmarshal, but rejects unknown
// object fields and anything after the first JSON value. Custom
// UnmarshalJSON methods use it to stay strict: the strictness of an outer
// decoder does not reach into them.
func Unmarshal(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return errors.New("unexpected data after the JSON value")
	}
	return nil
}

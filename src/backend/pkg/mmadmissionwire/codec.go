// Package mmadmissionwire implements the private MM installation protocol.
// Hashes bind data; only authenticated controller readback supplies authority.
package mmadmissionwire

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
)

const MaxFrame = 256 * 1024

var ErrInvalid = errors.New("invalid MM admission protocol")

// Decode rejects unknown fields, duplicate keys at every depth and trailing data.
func Decode(data []byte, target any) error {
	if len(data) == 0 || len(data) > MaxFrame {
		return ErrInvalid
	}
	scan := json.NewDecoder(bytes.NewReader(data))
	scan.UseNumber()
	if err := scanValue(scan, 0); err != nil {
		return ErrInvalid
	}
	if _, err := scan.Token(); err != io.EOF {
		return ErrInvalid
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(target); err != nil {
		return ErrInvalid
	}
	if err := requiredFields(data, reflect.TypeOf(target)); err != nil {
		return ErrInvalid
	}
	if v, ok := target.(interface{ Validate() error }); ok {
		return v.Validate()
	}
	return nil
}
func requiredFields(data []byte, t reflect.Type) error {
	for t.Kind() == reflect.Pointer {
		if bytes.Equal(data, []byte("null")) {
			return nil
		}
		t = t.Elem()
	}
	if t.Kind() == reflect.Struct {
		var fields map[string]json.RawMessage
		if json.Unmarshal(data, &fields) != nil {
			return ErrInvalid
		}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if !f.IsExported() {
				continue
			}
			tag := strings.Split(f.Tag.Get("json"), ",")[0]
			if tag == "-" {
				continue
			}
			if tag == "" {
				tag = f.Name
			}
			raw, ok := fields[tag]
			if !ok {
				return ErrInvalid
			}
			if requiredFields(raw, f.Type) != nil {
				return ErrInvalid
			}
		}
	}
	if t.Kind() == reflect.Slice {
		var items []json.RawMessage
		if json.Unmarshal(data, &items) != nil {
			return ErrInvalid
		}
		for _, raw := range items {
			if requiredFields(raw, t.Elem()) != nil {
				return ErrInvalid
			}
		}
	}
	return nil
}
func scanValue(d *json.Decoder, depth int) error {
	if depth > 32 {
		return ErrInvalid
	}
	t, e := d.Token()
	if e != nil {
		return e
	}
	delim, ok := t.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for d.More() {
			k, e := d.Token()
			if e != nil {
				return e
			}
			s, ok := k.(string)
			if !ok || seen[s] {
				return ErrInvalid
			}
			seen[s] = true
			if err := scanValue(d, depth+1); err != nil {
				return err
			}
		}
		t, e = d.Token()
		if e != nil || t != json.Delim('}') {
			return ErrInvalid
		}
	case '[':
		for d.More() {
			if err := scanValue(d, depth+1); err != nil {
				return err
			}
		}
		t, e = d.Token()
		if e != nil || t != json.Delim(']') {
			return ErrInvalid
		}
	default:
		return ErrInvalid
	}
	return nil
}

func ReadFrame(r io.Reader, target any) error {
	var h [4]byte
	if _, err := io.ReadFull(r, h[:]); err != nil {
		return ErrInvalid
	}
	n := binary.BigEndian.Uint32(h[:])
	if n == 0 || n > MaxFrame {
		return ErrInvalid
	}
	b := make([]byte, n)
	if _, err := io.ReadFull(r, b); err != nil {
		return ErrInvalid
	}
	return Decode(b, target)
}
func WriteFrame(w io.Writer, value any) error {
	b, err := json.Marshal(value)
	if err != nil || len(b) == 0 || len(b) > MaxFrame {
		return ErrInvalid
	}
	var out bytes.Buffer
	_ = binary.Write(&out, binary.BigEndian, uint32(len(b)))
	out.Write(b)
	_, err = io.Copy(w, &out)
	return err
}

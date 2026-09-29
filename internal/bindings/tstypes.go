package bindings

import (
	"encoding"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"
)

var (
	timeType          = reflect.TypeFor[time.Time]()
	rawMessageType    = reflect.TypeFor[json.RawMessage]()
	jsonMarshalerType = reflect.TypeFor[json.Marshaler]()
	textMarshalerType = reflect.TypeFor[encoding.TextMarshaler]()
)

// typeMapper converts Go types to TypeScript following encoding/json rules,
// collecting a declaration for every named struct it meets.
type typeMapper struct {
	decls map[string]string       // TS name -> declaration
	owner map[string]reflect.Type // TS name -> Go type, to detect collisions
}

func newTypeMapper() *typeMapper {
	return &typeMapper{decls: map[string]string{}, owner: map[string]reflect.Type{}}
}

// tsType returns the TypeScript type for t as encoding/json would encode it.
func (m *typeMapper) tsType(t reflect.Type) (string, error) {
	switch {
	case t == timeType:
		return "string", nil
	case t == rawMessageType:
		return "unknown", nil
	case t.Kind() != reflect.Pointer && t.Kind() != reflect.Interface &&
		(t.Implements(jsonMarshalerType) || reflect.PointerTo(t).Implements(jsonMarshalerType)):
		// Custom JSON encoding: the shape is not knowable from the type.
		return "unknown", nil
	case t.Kind() != reflect.Pointer && t.Kind() != reflect.Interface &&
		(t.Implements(textMarshalerType) || reflect.PointerTo(t).Implements(textMarshalerType)):
		return "string", nil
	}
	switch t.Kind() {
	case reflect.Bool:
		return "boolean", nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64:
		return "number", nil
	case reflect.String:
		return "string", nil
	case reflect.Interface:
		return "unknown", nil
	case reflect.Pointer:
		inner, err := m.tsType(t.Elem())
		if err != nil {
			return "", err
		}
		return inner + " | null", nil
	case reflect.Slice:
		if t.Elem().Kind() == reflect.Uint8 {
			return "string | null", nil // base64
		}
		elem, err := m.elemType(t.Elem())
		if err != nil {
			return "", err
		}
		return elem + "[] | null", nil
	case reflect.Array:
		elem, err := m.elemType(t.Elem())
		if err != nil {
			return "", err
		}
		return elem + "[]", nil
	case reflect.Map:
		switch t.Key().Kind() {
		case reflect.String, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		default:
			return "", fmt.Errorf("map key type %s cannot be encoded as JSON", t.Key())
		}
		elem, err := m.tsType(t.Elem())
		if err != nil {
			return "", err
		}
		return "Record<string, " + elem + "> | null", nil
	case reflect.Struct:
		if t.Name() == "" {
			return m.structBody(t, "")
		}
		return m.named(t)
	default:
		return "", fmt.Errorf("type %s cannot be encoded as JSON", t)
	}
}

// elemType parenthesizes union element types so "[]" binds to the union.
func (m *typeMapper) elemType(t reflect.Type) (string, error) {
	s, err := m.tsType(t)
	if err != nil {
		return "", err
	}
	if strings.Contains(s, " | ") {
		return "(" + s + ")", nil
	}
	return s, nil
}

func (m *typeMapper) named(t reflect.Type) (string, error) {
	name := exportedName(t.Name())
	if owner, ok := m.owner[name]; ok {
		if owner != t {
			return "", fmt.Errorf("two Go types map to TypeScript name %s: %s and %s", name, owner, t)
		}
		return name, nil
	}
	// Register before recursing so self-referential types terminate.
	m.owner[name] = t
	body, err := m.structBody(t, "")
	if err != nil {
		return "", err
	}
	m.decls[name] = "export interface " + name + " " + body
	return name, nil
}

// structBody renders "{ field: T; ... }" with the given indentation prefix.
func (m *typeMapper) structBody(t reflect.Type, indent string) (string, error) {
	var fields []string
	if err := m.collectFields(t, indent+"  ", &fields); err != nil {
		return "", err
	}
	if len(fields) == 0 {
		return "{}", nil
	}
	return "{\n" + strings.Join(fields, "\n") + "\n" + indent + "}", nil
}

func (m *typeMapper) collectFields(t reflect.Type, indent string, out *[]string) error {
	for i := range t.NumField() {
		f := t.Field(i)
		tag := f.Tag.Get("json")
		if tag == "-" {
			continue
		}
		name, opts, _ := strings.Cut(tag, ",")
		if f.Anonymous && name == "" {
			ft := f.Type
			if ft.Kind() == reflect.Pointer {
				ft = ft.Elem()
			}
			if ft.Kind() == reflect.Struct {
				if err := m.collectFields(ft, indent, out); err != nil {
					return err
				}
				continue
			}
		}
		if !f.IsExported() {
			continue
		}
		if name == "" {
			name = f.Name
		}
		var ts string
		var err error
		switch {
		case hasOpt(opts, "string") && isScalar(f.Type.Kind()):
			// encoding/json honors ",string" only on scalar fields.
			ts = "string"
		case f.Type.Kind() == reflect.Struct && f.Type.Name() == "":
			ts, err = m.structBody(f.Type, indent)
		default:
			ts, err = m.tsType(f.Type)
		}
		if err != nil {
			return fmt.Errorf("field %s: %w", f.Name, err)
		}
		optional := ""
		if hasOpt(opts, "omitempty") || hasOpt(opts, "omitzero") {
			optional = "?"
		}
		*out = append(*out, indent+propertyName(name)+optional+": "+ts+";")
	}
	return nil
}

// declarations returns all collected interfaces, sorted by name.
func (m *typeMapper) declarations() []string {
	names := make([]string, 0, len(m.decls))
	for n := range m.decls {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = m.decls[n]
	}
	return out
}

func hasOpt(opts, want string) bool {
	for _, o := range strings.Split(opts, ",") {
		if o == want {
			return true
		}
	}
	return false
}

// exportedName turns a Go type name into a TypeScript identifier. Generic
// instantiations such as "Page[example.com/x.Item]" become "Page_Item".
func exportedName(s string) string {
	if i := strings.IndexByte(s, '['); i >= 0 {
		args := strings.FieldsFunc(s[i+1:len(s)-1], func(r rune) bool { return r == ',' })
		s = s[:i]
		for _, a := range args {
			a = strings.TrimSpace(a)
			if j := strings.LastIndexByte(a, '.'); j >= 0 {
				a = a[j+1:]
			}
			s += "_" + a
		}
	}
	var b strings.Builder
	for _, r := range s {
		if r == '_' || r == '$' || ('a' <= r && r <= 'z') || ('A' <= r && r <= 'Z') || ('0' <= r && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	s = b.String()
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func isScalar(k reflect.Kind) bool {
	switch k {
	case reflect.Bool, reflect.String,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64:
		return true
	}
	return false
}

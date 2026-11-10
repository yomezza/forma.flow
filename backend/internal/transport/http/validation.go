package httptransport

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"github.com/yomezza/forma.flow/backend/internal/domain"
	"io"
	"math"
	"net/http"
	"net/mail"
	"reflect"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

//go:embed schemas.json
var schemaJSON []byte
var schemas map[string]any

func init() {
	if e := json.Unmarshal(schemaJSON, &schemas); e != nil {
		panic(e)
	}
}
func validateRef(name string, value any) error {
	return validateSchema(schemas[name].(map[string]any), value)
}
func validateSchema(s map[string]any, v any) error {
	if ref, ok := s["$ref"].(string); ok {
		return validateRef(strings.TrimPrefix(ref, "#/components/schemas/"), v)
	}
	if options, ok := s["oneOf"].([]any); ok {
		n := 0
		for _, o := range options {
			if validateSchema(o.(map[string]any), v) == nil {
				n++
			}
		}
		if n != 1 {
			return fmt.Errorf("oneOf")
		}
	}
	if options, ok := s["allOf"].([]any); ok {
		for _, o := range options {
			if e := validateSchema(o.(map[string]any), v); e != nil {
				return e
			}
		}
	}
	typ := s["type"]
	matches := func(t string) bool {
		switch t {
		case "null":
			return v == nil
		case "object":
			_, ok := v.(map[string]any)
			return ok
		case "array":
			_, ok := v.([]any)
			return ok
		case "string":
			_, ok := v.(string)
			return ok
		case "boolean":
			_, ok := v.(bool)
			return ok
		case "integer":
			n, ok := v.(float64)
			return ok && math.Trunc(n) == n && math.Abs(n) <= 9007199254740991
		case "number":
			_, ok := v.(float64)
			return ok
		}
		return false
	}
	if t, ok := typ.(string); ok && !matches(t) {
		return fmt.Errorf("type %s", t)
	}
	if ts, ok := typ.([]any); ok {
		found := false
		for _, t := range ts {
			if matches(t.(string)) {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("nullable type")
		}
	}
	if c, ok := s["const"]; ok && !reflect.DeepEqual(c, v) {
		return fmt.Errorf("const")
	}
	if enum, ok := s["enum"].([]any); ok {
		found := false
		for _, x := range enum {
			if reflect.DeepEqual(v, x) {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("enum")
		}
	}
	switch x := v.(type) {
	case map[string]any:
		if min, ok := s["minProperties"].(float64); ok && len(x) < int(min) {
			return fmt.Errorf("minProperties")
		}
		for _, r := range array(s["required"]) {
			if _, ok := x[r.(string)]; !ok {
				return fmt.Errorf("missing %s", r)
			}
		}
		props, _ := s["properties"].(map[string]any)
		for k, val := range x {
			if sub, ok := props[k].(map[string]any); ok {
				if e := validateSchema(sub, val); e != nil {
					return fmt.Errorf("%s: %w", k, e)
				}
			} else {
				switch additional := s["additionalProperties"].(type) {
				case bool:
					if !additional {
						return fmt.Errorf("unknown %s", k)
					}
				case map[string]any:
					if e := validateSchema(additional, val); e != nil {
						return e
					}
				}
			}
		}
	case []any:
		if len(x) > 1000 {
			return fmt.Errorf("array limit")
		}
		if min, ok := s["minItems"].(float64); ok && len(x) < int(min) {
			return fmt.Errorf("minItems")
		}
		seen := map[string]bool{}
		for _, val := range x {
			if sub, ok := s["items"].(map[string]any); ok {
				if e := validateSchema(sub, val); e != nil {
					return e
				}
			}
			if s["uniqueItems"] == true {
				b, _ := json.Marshal(val)
				if seen[string(b)] {
					return fmt.Errorf("duplicate")
				}
				seen[string(b)] = true
			}
		}
	case string:
		if min, ok := s["minLength"].(float64); ok && utf8.RuneCountInString(x) < int(min) {
			return fmt.Errorf("minLength")
		}
		if max, ok := s["maxLength"].(float64); ok && utf8.RuneCountInString(x) > int(max) {
			return fmt.Errorf("maxLength")
		}
		if p, ok := s["pattern"].(string); ok {
			if match, _ := regexp.MatchString(p, x); !match {
				return fmt.Errorf("pattern")
			}
		}
		switch s["format"] {
		case "date":
			d, e := time.Parse("2006-01-02", x)
			if e != nil || d.Format("2006-01-02") != x {
				return fmt.Errorf("date")
			}
		case "date-time":
			if _, e := time.Parse(time.RFC3339Nano, x); e != nil {
				return e
			}
		case "uuid":
			if match, _ := regexp.MatchString(`^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$`, x); !match {
				return fmt.Errorf("uuid")
			}
		case "email":
			a, e := mail.ParseAddress(x)
			if e != nil || a.Address != x {
				return fmt.Errorf("email")
			}
		}
	case float64:
		if min, ok := s["minimum"].(float64); ok && x < min {
			return fmt.Errorf("minimum")
		}
		if max, ok := s["maximum"].(float64); ok && x > max {
			return fmt.Errorf("maximum")
		}
	}
	return nil
}
func array(v any) []any { a, _ := v.([]any); return a }
func readBody(w http.ResponseWriter, r *http.Request, schema string) (domain.M, error) {
	if typ := strings.Split(r.Header.Get("Content-Type"), ";")[0]; typ != "application/json" {
		return nil, domain.Fail("VALIDATION", 400, "Требуется application/json.")
	}
	limit := int64(256 << 10)
	if r.URL.Path == "/api/v1/booking-requests" {
		limit = 32 << 10
	}
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	d := json.NewDecoder(r.Body)
	var m map[string]any
	if e := d.Decode(&m); e != nil {
		return nil, domain.Fail("VALIDATION", 400, "Некорректный JSON.")
	}
	var trailing any
	if e := d.Decode(&trailing); e != io.EOF {
		return nil, domain.Fail("VALIDATION", 400, "Ожидается один JSON-объект.")
	}
	if m == nil {
		return nil, domain.Validation("body")
	}
	if e := validateRef(schema, m); e != nil {
		return nil, domain.Validation(e.Error())
	}
	return domain.M(m), nil
}

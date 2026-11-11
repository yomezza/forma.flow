package domain

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

type M map[string]any

func (m M) String(k string) string { v, _ := m[k].(string); return v }
func (m M) Int(k string) int {
	switch v := m[k].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	case json.Number:
		n, _ := v.Int64()
		return int(n)
	}
	return 0
}
func (m M) Bool(k string) bool { v, _ := m[k].(bool); return v }
func (m M) Object(k string) M {
	switch v := m[k].(type) {
	case M:
		return v
	case map[string]any:
		return M(v)
	}
	return nil
}
func (m M) Strings(k string) []string {
	a := []string{}
	switch v := m[k].(type) {
	case []string:
		return v
	case []any:
		for _, x := range v {
			if s, ok := x.(string); ok {
				a = append(a, s)
			}
		}
	}
	return a
}
func (m M) Has(k string) bool { _, ok := m[k]; return ok }
func ID() string {
	b := make([]byte, 16)
	rand.Read(b)
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	h := hex.EncodeToString(b)
	return h[:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
}
func Text(s string, min, max int) bool {
	n := utf8.RuneCountInString(strings.TrimSpace(s))
	return n >= min && n <= max
}

var phoneRE = regexp.MustCompile(`^\+7[0-9]{10}$`)

func Phone(v string) (string, error) {
	s := strings.NewReplacer(" ", "", "(", "", ")", "", "-", "").Replace(v)
	if len(s) == 10 {
		s = "+7" + s
	} else if len(s) == 11 && (s[0] == '7' || s[0] == '8') {
		s = "+7" + s[1:]
	}
	if !phoneRE.MatchString(s) {
		return "", Validation("phone")
	}
	return s, nil
}
func Identity(in M) (string, string, error) {
	n := strings.Join(strings.Fields(in.String("fullName")), " ")
	if !Text(n, 3, 120) || len(strings.Fields(n)) < 2 {
		return "", "", Validation("fullName")
	}
	p, e := Phone(in.String("phone"))
	return n, p, e
}

type Actor struct {
	AccountID, SpecialistID string
	Roles                   []string
	MustChangePassword      bool
	SessionID, Token        string
	ExpiresAt               time.Time
}

func (a Actor) Has(role string) bool {
	for _, r := range a.Roles {
		if r == role {
			return true
		}
	}
	return false
}
func (a Actor) Admin() error {
	if a.AccountID == "" {
		return AuthRequired
	}
	if a.MustChangePassword {
		return Fail("FIRST_LOGIN_PASSWORD_CHANGE_REQUIRED", 403, "Сначала смените временный пароль.")
	}
	if !a.Has("admin") && !a.Has("owner") {
		return Forbidden
	}
	return nil
}
func (a Actor) Specialist() error {
	if a.AccountID == "" {
		return AuthRequired
	}
	if a.MustChangePassword {
		return Fail("FIRST_LOGIN_PASSWORD_CHANGE_REQUIRED", 403, "Сначала смените временный пароль.")
	}
	if !a.Has("specialist") || a.SpecialistID == "" {
		return Forbidden
	}
	return nil
}
func Clone(m M) M { b, _ := json.Marshal(m); var n M; json.Unmarshal(b, &n); return n }

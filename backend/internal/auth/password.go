package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"golang.org/x/crypto/argon2"
	"strings"
)

func Hash(password string) (string, error) {
	salt := make([]byte, 16)
	if _, e := rand.Read(salt); e != nil {
		return "", e
	}
	key := argon2.IDKey([]byte(password), salt, 3, 64*1024, 2, 32)
	return "$argon2id$v=19$m=65536,t=3,p=2$" + base64.RawStdEncoding.EncodeToString(salt) + "$" + base64.RawStdEncoding.EncodeToString(key), nil
}
func Verify(hash, password string) bool {
	if len(password) > 1024 {
		return false
	}
	p := strings.Split(hash, "$")
	if len(p) != 6 || p[1] != "argon2id" || p[2] != "v=19" {
		return false
	}
	var m, t uint32
	var parallel uint8
	if _, e := fmt.Sscanf(p[3], "m=%d,t=%d,p=%d", &m, &t, &parallel); e != nil || m < 8192 || m > 128*1024 || t < 1 || t > 5 || parallel < 1 || parallel > 4 {
		return false
	}
	salt, e := base64.RawStdEncoding.DecodeString(p[4])
	if e != nil || len(salt) != 16 {
		return false
	}
	key, e := base64.RawStdEncoding.DecodeString(p[5])
	if e != nil || len(key) != 32 {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, t, m, parallel, 32)
	return subtle.ConstantTimeCompare(got, key) == 1
}

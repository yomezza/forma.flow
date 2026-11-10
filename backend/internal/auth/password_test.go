package auth

import "testing"

func TestArgon2id(t *testing.T) {
	h, e := Hash("a-long-test-password")
	if e != nil {
		t.Fatal(e)
	}
	if !Verify(h, "a-long-test-password") || Verify(h, "wrong") {
		t.Fatal("password verification")
	}
	for _, bad := range []string{"plaintext", "$argon2id$v=19$m=4294967295,t=3,p=2$bad$bad"} {
		if Verify(bad, "x") {
			t.Fatal("invalid hash accepted")
		}
	}
}

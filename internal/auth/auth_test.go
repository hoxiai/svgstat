package auth

import "testing"

func TestHashTokenDoesNotStorePlaintext(t *testing.T) {
	token := "session-secret"
	hashed := hashToken(token)
	if hashed == token || len(hashed) != 64 {
		t.Fatalf("hashToken() = %q", hashed)
	}
	if hashToken(token) != hashed || hashToken("another") == hashed {
		t.Fatal("session token hashing is not stable and distinct")
	}
}

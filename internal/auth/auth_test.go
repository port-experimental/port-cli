package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestParseToken(t *testing.T) {
	exp := time.Now().Unix()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"aud":                             "https://api.example.com",
		"exp":                             float64(exp),
		"https://api.example.com/email":   "user@test.com",
		"https://api.example.com/orgId":   "someOrgId",
		"https://api.example.com/orgName": "Org Name",
	})
	ss, err := token.SignedString([]byte("signing-key"))
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseToken(ss)
	if err != nil {
		t.Fatal(err)
	}

	if aud := parsed.Claims.Audience; aud != "https://api.example.com" {
		t.Errorf("expected audience https://api.example.com but got %v", aud)
	}
	if email := parsed.Claims.Email; email != "user@test.com" {
		t.Errorf("expected email user@test.com but got %v", email)
	}
	if orgId := parsed.Claims.OrgId; orgId != "someOrgId" {
		t.Errorf("expected orgId someOrgId but got %v", orgId)
	}
	if orgName := parsed.Claims.OrgName; orgName != "Org Name" {
		t.Errorf("expected orgName Org Name but got %v", orgName)
	}
	if exp != parsed.Claims.Expiry.Unix() {
		t.Errorf("expected expiry %v, got '%v'", exp, parsed.Claims.Expiry.Unix())
	}
}

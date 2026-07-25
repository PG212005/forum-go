package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestParseRegistrationFormTrimsIdentityFields(t *testing.T) {
	values := url.Values{
		"email":    {"  student@example.com  "},
		"username": {"  student  "},
		"password": {" password with spaces "},
	}
	request := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(values.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	got := parseRegistrationForm(request)

	if got.email != "student@example.com" || got.username != "student" {
		t.Fatalf("identity fields were not trimmed: %#v", got)
	}
	if got.password != " password with spaces " {
		t.Fatalf("password was unexpectedly changed: %q", got.password)
	}
}

func TestValidateRegistrationForm(t *testing.T) {
	valid := registrationForm{
		email:    "student@example.com",
		username: "student",
		password: "secret",
	}
	if err := validateRegistrationForm(valid); err != nil {
		t.Fatalf("valid form returned error: %v", err)
	}

	valid.email = ""
	if err := validateRegistrationForm(valid); err == nil {
		t.Fatal("empty email did not return an error")
	}
}

func TestSetSessionCookieUsesSecurityFlags(t *testing.T) {
	recorder := httptest.NewRecorder()
	expiresAt := time.Now().Add(time.Hour)

	setSessionCookie(recorder, session{token: "token", expiresAt: expiresAt})

	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookie count = %d, want 1", len(cookies))
	}
	cookie := cookies[0]
	if !cookie.HttpOnly {
		t.Fatal("session cookie must be HttpOnly")
	}
	if cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("SameSite = %v, want Lax", cookie.SameSite)
	}
	if cookie.Path != "/" {
		t.Fatalf("Path = %q, want /", cookie.Path)
	}
}

func TestClearSessionCookieExpiresImmediately(t *testing.T) {
	recorder := httptest.NewRecorder()

	clearSessionCookie(recorder)

	cookie := recorder.Result().Cookies()[0]
	if cookie.MaxAge != -1 {
		t.Fatalf("MaxAge = %d, want -1", cookie.MaxAge)
	}
	if !cookie.Expires.Before(time.Now()) {
		t.Fatalf("Expires = %v, want a past time", cookie.Expires)
	}
}

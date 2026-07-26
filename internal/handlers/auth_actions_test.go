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

	got, err := parseRegistrationForm(request)
	if err != nil {
		t.Fatalf("parseRegistrationForm() unexpected error: %v", err)
	}

	if got.email != "student@example.com" || got.username != "student" {
		t.Fatalf("identity fields were not trimmed: %#v", got)
	}
	if got.password != " password with spaces " {
		t.Fatalf("password was unexpectedly changed: %q", got.password)
	}
}

func TestParseLoginFormParsesPostBody(t *testing.T) {
	values := url.Values{
		"email":    {"  student@example.com  "},
		"password": {" password with spaces "},
	}
	request := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(values.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	got, err := parseLoginForm(request)
	if err != nil {
		t.Fatalf("parseLoginForm() unexpected error: %v", err)
	}
	if got.email != "student@example.com" {
		t.Fatalf("email = %q, want trimmed email", got.email)
	}
	if got.password != " password with spaces " {
		t.Fatalf("password was unexpectedly changed: %q", got.password)
	}
}

func TestParseLoginFormReturnsMalformedBodyError(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("%"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	if _, err := parseLoginForm(request); err == nil {
		t.Fatal("parseLoginForm() returned nil error for malformed form body")
	}
}

func TestValidateRegistrationForm(t *testing.T) {
	tests := []struct {
		name    string
		form    registrationForm
		wantErr bool
	}{
		{
			name: "valid",
			form: registrationForm{
				email:    "student@example.com",
				username: "student",
				password: "secret",
			},
		},
		{
			name:    "missing email",
			form:    registrationForm{username: "student", password: "secret"},
			wantErr: true,
		},
		{
			name:    "username contains only spaces",
			form:    registrationForm{email: "student@example.com", username: "   ", password: "secret"},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateRegistrationForm(test.form)
			if test.wantErr && err == nil {
				t.Fatal("validateRegistrationForm() returned nil error")
			}
			if !test.wantErr && err != nil {
				t.Fatalf("validateRegistrationForm() unexpected error: %v", err)
			}
		})
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

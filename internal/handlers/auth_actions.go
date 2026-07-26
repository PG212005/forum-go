package handlers

import (
	"database/sql"
	"errors"
	"forum/internal/database"
	"net/http"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	"golang.org/x/crypto/bcrypt"
)

const (
	sessionCookieName = "session_token"
	sessionDuration   = time.Hour
)

var errInvalidCredentials = errors.New("invalid credentials")

type registrationForm struct {
	email    string
	username string
	password string
}

type loginForm struct {
	email    string
	password string
}

type session struct {
	token     string
	expiresAt time.Time
}

func renderAuthPage(w http.ResponseWriter, page, errorMessage string, statusCode int) {
	if statusCode != http.StatusOK {
		w.WriteHeader(statusCode)
	}
	data := PageData{IsLoggedIn: false, Error: errorMessage}
	if err := renderTemplate(w, page, data); err != nil && statusCode == http.StatusOK {
		ErrorPage(w, http.StatusInternalServerError, "500 - Template Error")
	}
}

func parseRegistrationForm(r *http.Request) (registrationForm, error) {
	if err := r.ParseForm(); err != nil {
		return registrationForm{}, err
	}

	return registrationForm{
		email:    strings.TrimSpace(r.PostForm.Get("email")),
		username: strings.TrimSpace(r.PostForm.Get("username")),
		password: r.PostForm.Get("password"),
	}, nil
}

func validateRegistrationForm(form registrationForm) error {
	if strings.TrimSpace(form.email) == "" ||
		strings.TrimSpace(form.username) == "" ||
		form.password == "" {
		return errors.New("All fields are required")
	}
	return nil
}

func createUser(form registrationForm) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(form.password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = database.DB.Exec(
		"INSERT INTO users (email, username, password) VALUES (?, ?, ?)",
		form.email,
		form.username,
		hashedPassword,
	)
	return err
}

// RegisterUser serves the registration form (GET) and creates users (POST).
func RegisterUser(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		renderAuthPage(w, "ui/html/register.page.html", "", http.StatusOK)
	case http.MethodPost:
		handleRegistration(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleRegistration(w http.ResponseWriter, r *http.Request) {
	form, err := parseRegistrationForm(r)
	if err != nil {
		renderAuthPage(w, "ui/html/register.page.html", "Invalid form submission", http.StatusBadRequest)
		return
	}
	if err := validateRegistrationForm(form); err != nil {
		renderAuthPage(w, "ui/html/register.page.html", err.Error(), http.StatusBadRequest)
		return
	}
	if err := createUser(form); err != nil {
		renderAuthPage(w, "ui/html/register.page.html", "Username or Email already taken", http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func parseLoginForm(r *http.Request) (loginForm, error) {
	if err := r.ParseForm(); err != nil {
		return loginForm{}, err
	}

	return loginForm{
		email:    strings.TrimSpace(r.PostForm.Get("email")),
		password: r.PostForm.Get("password"),
	}, nil
}

func authenticateUser(form loginForm) (int, error) {
	var userID int
	var passwordHash string
	err := database.DB.QueryRow(
		"SELECT id, password FROM users WHERE email = ?",
		form.email,
	).Scan(&userID, &passwordHash)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, errInvalidCredentials
	}
	if err != nil {
		return 0, err
	}
	if bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(form.password)) != nil {
		return 0, errInvalidCredentials
	}
	return userID, nil
}

func replaceUserSession(userID int) (session, error) {
	token, err := uuid.NewV4()
	if err != nil {
		return session{}, err
	}
	newSession := session{
		token:     token.String(),
		expiresAt: time.Now().Add(sessionDuration),
	}

	tx, err := database.DB.Begin()
	if err != nil {
		return session{}, err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM sessions WHERE user_id = ?", userID); err != nil {
		return session{}, err
	}
	if _, err := tx.Exec(
		"INSERT INTO sessions (uuid, user_id, expires_at) VALUES (?, ?, ?)",
		newSession.token,
		userID,
		newSession.expiresAt,
	); err != nil {
		return session{}, err
	}
	if err := tx.Commit(); err != nil {
		return session{}, err
	}
	return newSession, nil
}

func setSessionCookie(w http.ResponseWriter, current session) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    current.token,
		Expires:  current.expiresAt,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// LoginUser serves the login form (GET) and authenticates users (POST).
func LoginUser(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		renderAuthPage(w, "ui/html/login.page.html", "", http.StatusOK)
	case http.MethodPost:
		handleLogin(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	form, err := parseLoginForm(r)
	if err != nil {
		renderAuthPage(w, "ui/html/login.page.html", "Invalid form submission", http.StatusBadRequest)
		return
	}

	userID, err := authenticateUser(form)
	if errors.Is(err, errInvalidCredentials) {
		renderAuthPage(w, "ui/html/login.page.html", "Invalid email or password", http.StatusBadRequest)
		return
	}
	if err != nil {
		ErrorPage(w, http.StatusInternalServerError, "500 - Internal Server Error")
		return
	}

	currentSession, err := replaceUserSession(userID)
	if err != nil {
		ErrorPage(w, http.StatusInternalServerError, "500 - Session error")
		return
	}
	setSessionCookie(w, currentSession)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// LogoutUser invalidates the current session and clears the session cookie.
func LogoutUser(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		_, _ = database.DB.Exec("DELETE FROM sessions WHERE uuid = ?", cookie.Value)
	}
	clearSessionCookie(w)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

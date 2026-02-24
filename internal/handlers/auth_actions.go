package handlers

import (
	"database/sql"
	"forum/internal/database"
	"html/template"
	"net/http"
	"time"

	"github.com/gofrs/uuid"
	"golang.org/x/crypto/bcrypt"
)

// RegisterUser handles user registration
func RegisterUser(w http.ResponseWriter, r *http.Request) {
	// 1. GET Request: Εμφάνισε τη φόρμα
	if r.Method == http.MethodGet {
		tmpl, err := template.ParseFiles("ui/html/base.layout.html", "ui/html/register.page.html")
		if err != nil {
			http.Error(w, "Template error", http.StatusInternalServerError)
			return
		}
		data := PageData{IsLoggedIn: false}
		tmpl.Execute(w, data)
		return
	}

	// 2. POST Request: Κάνε την εγγραφή
	if r.Method == http.MethodPost {
		email := r.FormValue("email")
		username := r.FormValue("username")
		password := r.FormValue("password")

		if email == "" || username == "" || password == "" {
			http.Error(w, "All fields are required", http.StatusBadRequest)
			return
		}

		hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

		_, err := database.DB.Exec("INSERT INTO users (email, username, password) VALUES (?, ?, ?)", email, username, hashedPassword)
		if err != nil {
			http.Error(w, "Username or Email already taken", http.StatusBadRequest)
			return
		}

		http.Redirect(w, r, "/login", http.StatusSeeOther)
	}
}

// LoginUser handles user login
func LoginUser(w http.ResponseWriter, r *http.Request) {
	// 1. GET Request: Εμφάνισε τη φόρμα
	if r.Method == http.MethodGet {
		tmpl, err := template.ParseFiles("ui/html/base.layout.html", "ui/html/login.page.html")
		if err != nil {
			http.Error(w, "Template error", http.StatusInternalServerError)
			return
		}
		data := PageData{IsLoggedIn: false}
		tmpl.Execute(w, data)
		return
	}

	// 2. POST Request: Κάνε Login
	if r.Method == http.MethodPost {
		email := r.FormValue("email")
		password := r.FormValue("password")

		var id int
		var storedPassword string
		err := database.DB.QueryRow("SELECT id, password FROM users WHERE email = ?", email).Scan(&id, &storedPassword)

		if err == sql.ErrNoRows || bcrypt.CompareHashAndPassword([]byte(storedPassword), []byte(password)) != nil {
			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
			return
		}

		// Διαγραφή παλιών sessions
		database.DB.Exec("DELETE FROM sessions WHERE user_id = ?", id)

		// Νέο session
		sessionToken, _ := uuid.NewV4()
		expiresAt := time.Now().Add(1 * time.Hour)

		database.DB.Exec("INSERT INTO sessions (uuid, user_id, expires_at) VALUES (?, ?, ?)", sessionToken.String(), id, expiresAt)

		http.SetCookie(w, &http.Cookie{
			Name:    "session_token",
			Value:   sessionToken.String(),
			Expires: expiresAt,
			Path:    "/",
		})

		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

// LogoutUser handles logout
func LogoutUser(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("session_token")
	if err == nil {
		database.DB.Exec("DELETE FROM sessions WHERE uuid = ?", c.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:    "session_token",
		Value:   "",
		Expires: time.Now().Add(-1 * time.Hour),
		Path:    "/",
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

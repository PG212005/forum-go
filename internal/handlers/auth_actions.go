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
	// 1. GET Request
	if r.Method == http.MethodGet {
		tmpl, err := template.ParseFiles("ui/html/base.layout.html", "ui/html/register.page.html")
		if err != nil {
			ErrorPage(w, http.StatusInternalServerError, "500 - Template Error")
			return
		}
		data := PageData{IsLoggedIn: false}
		if err := tmpl.Execute(w, data); err != nil {
			ErrorPage(w, http.StatusInternalServerError, "500 - Template Error")
		}
		return
	}

	// 2. POST Request
	if r.Method == http.MethodPost {
		email := r.FormValue("email")
		username := r.FormValue("username")
		password := r.FormValue("password")

		// Έλεγχος κενών
		if email == "" || username == "" || password == "" {
			w.WriteHeader(http.StatusBadRequest)
			tmpl, err := template.ParseFiles("ui/html/base.layout.html", "ui/html/register.page.html")
			if err != nil {
				return
			}
			_ = tmpl.Execute(w, PageData{IsLoggedIn: false, Error: "All fields are required"})
			return
		}

		hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

		_, err := database.DB.Exec("INSERT INTO users (email, username, password) VALUES (?, ?, ?)", email, username, hashedPassword)
		if err != nil {
			// Ορίζουμε το Status Code 400 (Bad Request) ή 409 (Conflict)
			// πριν το Execute για να το καταγράψει ο browser/audit tool.
			w.WriteHeader(http.StatusBadRequest)

			tmpl, err := template.ParseFiles("ui/html/base.layout.html", "ui/html/register.page.html")
			if err != nil {
				return
			}

			// Περνάμε το μήνυμα "Username or Email already taken" στο template
			data := PageData{
				IsLoggedIn: false,
				Error:      "Username or Email already taken",
			}
			_ = tmpl.Execute(w, data)
			return
		}

		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// LoginUser handles user login
// LoginUser handles user login
func LoginUser(w http.ResponseWriter, r *http.Request) {
	// 1. GET Request: Show Form
	if r.Method == http.MethodGet {
		tmpl, err := template.ParseFiles("ui/html/base.layout.html", "ui/html/login.page.html")
		if err != nil {
			ErrorPage(w, http.StatusInternalServerError, "500 - Template Error")
			return
		}
		data := PageData{IsLoggedIn: false}
		if err := tmpl.Execute(w, data); err != nil {
			ErrorPage(w, http.StatusInternalServerError, "500 - Template Error")
		}
		return
	}

	// 2. POST Request: Process Login
	if r.Method == http.MethodPost {
		email := r.FormValue("email")
		password := r.FormValue("password")

		var id int
		var storedPassword string
		err := database.DB.QueryRow("SELECT id, password FROM users WHERE email = ?", email).Scan(&id, &storedPassword)
		if err != nil && err != sql.ErrNoRows {
			ErrorPage(w, http.StatusInternalServerError, "500 - Internal Server Error")
			return
		}

		// ΕΔΩ ΕΙΝΑΙ Η ΑΛΛΑΓΗ:
		if err == sql.ErrNoRows || bcrypt.CompareHashAndPassword([]byte(storedPassword), []byte(password)) != nil {
			w.WriteHeader(http.StatusBadRequest)
			// Αντί για http.Error, ξαναφορτώνουμε τη σελίδα με μήνυμα Error
			tmpl, err := template.ParseFiles("ui/html/base.layout.html", "ui/html/login.page.html")
			if err != nil {
				return
			}
			data := PageData{
				IsLoggedIn: false,
				Error:      "Invalid email or password", // Το μήνυμα λάθους
			}
			_ = tmpl.Execute(w, data)
			return
		}

		// ... (Ο υπόλοιπος κώδικας για το Session παραμένει ίδιος) ...
		database.DB.Exec("DELETE FROM sessions WHERE user_id = ?", id)
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
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// LogoutUser handles logout
func LogoutUser(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("session_token")
	if err == nil {
		database.DB.Exec("DELETE FROM sessions WHERE uuid = ?", c.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    "",
		Expires:  time.Now().Add(-1 * time.Hour),
		Path:     "/",
		HttpOnly: true, // <--- ΠΡΟΣΘΗΚΗ: Το JavaScript δεν μπορεί να διαβάσει το cookie
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

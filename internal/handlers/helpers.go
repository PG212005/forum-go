package handlers

import (
	"forum/internal/database"
	"forum/internal/models"
	"html/template"
	"net/http"
	"time"
)

// GetUserFromSession returns the user ID and bool if logged in
func GetUserFromSession(r *http.Request) (models.User, bool) {
	c, err := r.Cookie("session_token")
	if err != nil {
		return models.User{}, false
	}

	var user models.User
	var expiresAt time.Time

	// Join tables to get user info from session
	query := `SELECT u.id, u.username, s.expires_at 
			  FROM sessions s 
			  JOIN users u ON s.user_id = u.id 
			  WHERE s.uuid = ?`

	err = database.DB.QueryRow(query, c.Value).Scan(&user.ID, &user.Username, &expiresAt)
	if err != nil {
		return models.User{}, false
	}

	if time.Now().After(expiresAt) {
		return models.User{}, false
	}

	return user, true
}

// ErrorPage shows a nice error page
func ErrorPage(w http.ResponseWriter, statusCode int, message string) {
	w.WriteHeader(statusCode) // Σημαντικό για το Audit (να επιστρέφει σωστό status code)

	tmpl, err := template.ParseFiles("ui/html/base.layout.html", "ui/html/error.page.html")
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := PageData{
		Error: message, // Χρησιμοποιούμε το πεδίο Error για τον τίτλο (π.χ. "404")
	}
	tmpl.Execute(w, data)
}

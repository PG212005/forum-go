package handlers

import (
	"fmt"
	"forum/internal/database"
	"forum/internal/models"
	"html/template"
	"log"
	"net/http"
	"sync"
	"time"
)

var (
	templateCache   = map[string]*template.Template{}
	templateCacheMu sync.RWMutex
)

// GetUserFromSession resolves the currently logged-in user from the session cookie.
// It returns false when the cookie is missing, invalid, or expired.
func GetUserFromSession(r *http.Request) (models.User, bool) {
	c, err := r.Cookie(sessionCookieName)
	if err != nil {
		return models.User{}, false
	}

	// Έλεγχος αν η βάση είναι nil για αποφυγή panic
	if database.DB == nil {
		log.Println("Database connection is nil")
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

// getTemplate returns a cached template when available, otherwise parses and caches it.
func getTemplate(page string) (*template.Template, error) {
	templateCacheMu.RLock()
	tmpl, ok := templateCache[page]
	templateCacheMu.RUnlock()
	if ok {
		return tmpl, nil
	}

	parsed, err := template.ParseFiles("ui/html/base.layout.html", page)
	if err != nil {
		return nil, err
	}

	templateCacheMu.Lock()
	if cached, ok := templateCache[page]; ok {
		templateCacheMu.Unlock()
		return cached, nil
	}
	templateCache[page] = parsed
	templateCacheMu.Unlock()

	return parsed, nil
}

// renderTemplate renders a page template using the common base layout.
func renderTemplate(w http.ResponseWriter, page string, data PageData) error {
	tmpl, err := getTemplate(page)
	if err != nil {
		return err
	}
	return tmpl.Execute(w, data)
}

// ErrorPage renders a consistent error page for user-facing failures.
func ErrorPage(w http.ResponseWriter, statusCode int, message string) {
	// 1. Ορίζουμε το status code ΠΡΙΝ από οτιδήποτε άλλο (Κρίσιμο για το Audit)
	w.WriteHeader(statusCode)

	// 2. Προσπάθεια για parsing των templates
	tmpl, err := getTemplate("ui/html/error.page.html")
	if err != nil {
		// ΔΙΟΡΘΩΣΗ: Αν αποτύχει το template, ΔΕΝ ξανακαλούμε την ErrorPage.
		// Στέλνουμε ένα απλό κείμενο ως έσχατη λύση για να αποφύγουμε το crash.
		log.Printf("CRITICAL: Template Error in ErrorPage: %v", err)
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintf(w, "Error %d: %s (Template missing)", statusCode, message)
		return
	}

	// 3. Προετοιμασία δεδομένων
	// Χρησιμοποιούμε το Error πεδίο της PageData
	data := PageData{
		Error:      message,
		IsLoggedIn: false,
	}

	// 4. Εκτέλεση
	err = tmpl.Execute(w, data)
	if err != nil {
		log.Printf("Execute Error in ErrorPage: %v", err)
	}
}

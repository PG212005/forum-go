package middleware

import (
	"forum/internal/handlers"
	"net/http"
)

// RequireLogin ελέγχει αν ο χρήστης είναι συνδεδεμένος. Αν όχι, τον στέλνει στο Login.
func RequireLogin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, isLoggedIn := handlers.GetUserFromSession(r)
		if !isLoggedIn {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}

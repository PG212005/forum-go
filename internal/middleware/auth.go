package middleware

import (
	"forum/internal/handlers"
	"net/http"
)

// RequireLogin ensures that protected handlers only run for authenticated users.
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

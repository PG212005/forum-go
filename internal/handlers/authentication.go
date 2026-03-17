package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"forum/internal/database"
	"net/http"
	"os"
	"time" // ΠΡΟΣΘΗΚΗ

	"github.com/gofrs/uuid" // ΠΡΟΣΘΗΚΗ
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// Ρυθμίσεις OAuth από το .env
func getGoogleConfig() *oauth2.Config {
	return &oauth2.Config{
		RedirectURL:  "http://localhost:8080/auth/google/callback",
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email", "https://www.googleapis.com/auth/userinfo.profile"},
		Endpoint:     google.Endpoint,
	}
}

func getGithubConfig() *oauth2.Config {
	return &oauth2.Config{
		RedirectURL:  "http://localhost:8080/auth/github/callback",
		ClientID:     os.Getenv("GITHUB_CLIENT_ID"),
		ClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"),
		Scopes:       []string{"user:email"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://github.com/login/oauth/authorize",
			TokenURL: "https://github.com/login/oauth/access_token",
		},
	}
}

// HANDLERS ΓΙΑ GOOGLE
func GoogleLogin(w http.ResponseWriter, r *http.Request) {
	url := getGoogleConfig().AuthCodeURL("state-token")
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func GoogleCallback(w http.ResponseWriter, r *http.Request) {
	code := r.FormValue("code")
	if code == "" {
		http.Error(w, "Missing authorization code", http.StatusBadRequest)
		return
	}

	token, err := getGoogleConfig().Exchange(context.Background(), code)
	if err != nil {
		http.Error(w, "Failed to exchange token", http.StatusBadRequest)
		return
	}

	resp, err := http.Get("https://www.googleapis.com/oauth2/v2/userinfo?access_token=" + token.AccessToken)
	if err != nil {
		http.Error(w, "Failed to fetch user info", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		http.Error(w, "Failed to fetch user info", http.StatusInternalServerError)
		return
	}

	var userInfo struct {
		ID    string `json:"id"`
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		http.Error(w, "Failed to decode user info", http.StatusInternalServerError)
		return
	}
	if userInfo.Email == "" || userInfo.ID == "" {
		http.Error(w, "Incomplete user info", http.StatusBadRequest)
		return
	}

	handleOAuthUser(w, r, userInfo.Email, userInfo.Name, userInfo.ID, "google")
}

// HANDLERS ΓΙΑ GITHUB
func GithubLogin(w http.ResponseWriter, r *http.Request) {
	url := getGithubConfig().AuthCodeURL("state-token")
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func GithubCallback(w http.ResponseWriter, r *http.Request) {
	code := r.FormValue("code")
	if code == "" {
		http.Error(w, "Missing authorization code", http.StatusBadRequest)
		return
	}

	token, err := getGithubConfig().Exchange(context.Background(), code)
	if err != nil {
		http.Error(w, "Failed to exchange GitHub token", http.StatusBadRequest)
		return
	}

	client := getGithubConfig().Client(context.Background(), token)
	resp, err := client.Get("https://api.github.com/user")
	if err != nil {
		http.Error(w, "Failed to fetch GitHub user info", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		http.Error(w, "Failed to fetch GitHub user info", http.StatusInternalServerError)
		return
	}

	var userInfo struct {
		ID    int    `json:"id"`
		Login string `json:"login"`
		Email string `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		http.Error(w, "Failed to decode GitHub user info", http.StatusInternalServerError)
		return
	}
	if userInfo.ID == 0 {
		http.Error(w, "Incomplete GitHub user info", http.StatusBadRequest)
		return
	}

	handleOAuthUser(w, r, userInfo.Email, userInfo.Login, fmt.Sprintf("%d", userInfo.ID), "github")
}

// ΚΟΙΝΗ ΛΟΓΙΚΗ ΓΙΑ ΟΛΟΥΣ ΤΟΥΣ PROVIDERS
func handleOAuthUser(w http.ResponseWriter, r *http.Request, email, username, providerID, providerName string) {
	var userID int
	query := fmt.Sprintf("SELECT id FROM users WHERE %s_id = ? OR email = ?", providerName)
	err := database.DB.QueryRow(query, providerID, email).Scan(&userID)

	if err == sql.ErrNoRows {
		res, err := database.DB.Exec(
			fmt.Sprintf("INSERT INTO users (email, username, %s_id) VALUES (?, ?, ?)", providerName),
			email, username, providerID,
		)
		if err != nil {
			// Fail-safe αν το username υπάρχει ήδη
			res, err = database.DB.Exec(
				fmt.Sprintf("INSERT INTO users (email, username, %s_id) VALUES (?, ?, ?)", providerName),
				email, email, providerID,
			)
			if err != nil {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
		}
		lastID, _ := res.LastInsertId()
		userID = int(lastID)
	} else if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	} else {
		if _, err := database.DB.Exec(fmt.Sprintf("UPDATE users SET %s_id = ? WHERE id = ?", providerName), providerID, userID); err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	}

	createSessionAndRedirect(w, r, userID)
}

// ΣΥΝΑΡΤΗΣΗ SESSION (ΠΡΕΠΕΙ ΝΑ ΥΠΑΡΧΕΙ ΣΤΟ ΑΡΧΕΙΟ)
func createSessionAndRedirect(w http.ResponseWriter, r *http.Request, userID int) {
	if _, err := database.DB.Exec("DELETE FROM sessions WHERE user_id = ?", userID); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	sessionToken, _ := uuid.NewV4()
	expiresAt := time.Now().Add(1 * time.Hour)

	_, err := database.DB.Exec("INSERT INTO sessions (uuid, user_id, expires_at) VALUES (?, ?, ?)",
		sessionToken.String(), userID, expiresAt)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    sessionToken.String(),
		Expires:  expiresAt,
		Path:     "/",
		HttpOnly: true,
	})

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

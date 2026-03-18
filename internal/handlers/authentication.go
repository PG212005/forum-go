package handlers

import (
	"bytes"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"forum/internal/database"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time" // ΠΡΟΣΘΗΚΗ

	"github.com/gofrs/uuid" // ΠΡΟΣΘΗΚΗ
)

type oauthProviderConfig struct {
	Name         string
	RedirectURL  string
	ClientID     string
	ClientSecret string
	AuthURL      string
	TokenURL     string
	Scopes       []string
	UserInfoURL  string
}

type oauthTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
}

func getGoogleConfig() oauthProviderConfig {
	return oauthProviderConfig{
		Name:         "google",
		RedirectURL:  "http://localhost:8080/auth/google/callback",
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		AuthURL:      "https://accounts.google.com/o/oauth2/v2/auth",
		TokenURL:     "https://oauth2.googleapis.com/token",
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		UserInfoURL: "https://www.googleapis.com/oauth2/v2/userinfo",
	}
}

func getGithubConfig() oauthProviderConfig {
	return oauthProviderConfig{
		Name:         "github",
		RedirectURL:  "http://localhost:8080/auth/github/callback",
		ClientID:     os.Getenv("GITHUB_CLIENT_ID"),
		ClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"),
		AuthURL:      "https://github.com/login/oauth/authorize",
		TokenURL:     "https://github.com/login/oauth/access_token",
		Scopes:       []string{"user:email"},
		UserInfoURL:  "https://api.github.com/user",
	}
}

func newOAuthState() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", buf), nil
}

func buildOAuthAuthURL(cfg oauthProviderConfig, state string) string {
	query := url.Values{}
	query.Set("client_id", cfg.ClientID)
	query.Set("redirect_uri", cfg.RedirectURL)
	query.Set("response_type", "code")
	query.Set("state", state)
	if len(cfg.Scopes) > 0 {
		query.Set("scope", joinScopes(cfg.Scopes))
	}

	return cfg.AuthURL + "?" + query.Encode()
}

func joinScopes(scopes []string) string {
	if len(scopes) == 0 {
		return ""
	}
	result := scopes[0]
	for i := 1; i < len(scopes); i++ {
		result += " " + scopes[i]
	}
	return result
}

func startOAuthLogin(w http.ResponseWriter, r *http.Request, cfg oauthProviderConfig) {
	if cfg.ClientID == "" || cfg.ClientSecret == "" {
		http.Error(w, "OAuth provider is not configured", http.StatusServiceUnavailable)
		return
	}

	state, err := newOAuthState()
	if err != nil {
		http.Error(w, "Failed to initialize OAuth state", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     cfg.Name + "_oauth_state",
		Value:    state,
		Path:     "/",
		HttpOnly: true,
		Expires:  time.Now().Add(10 * time.Minute),
	})

	http.Redirect(w, r, buildOAuthAuthURL(cfg, state), http.StatusTemporaryRedirect)
}

func validateOAuthState(r *http.Request, providerName string) bool {
	state := r.FormValue("state")
	if state == "" {
		return false
	}

	cookie, err := r.Cookie(providerName + "_oauth_state")
	if err != nil {
		return false
	}

	return cookie.Value == state
}

func exchangeCodeForToken(cfg oauthProviderConfig, code string) (oauthTokenResponse, error) {
	form := url.Values{}
	form.Set("client_id", cfg.ClientID)
	form.Set("client_secret", cfg.ClientSecret)
	form.Set("code", code)
	form.Set("redirect_uri", cfg.RedirectURL)
	form.Set("grant_type", "authorization_code")

	req, err := http.NewRequest(http.MethodPost, cfg.TokenURL, bytes.NewBufferString(form.Encode()))
	if err != nil {
		return oauthTokenResponse{}, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return oauthTokenResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return oauthTokenResponse{}, fmt.Errorf("token exchange failed: %s", string(body))
	}

	var token oauthTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return oauthTokenResponse{}, err
	}
	if token.AccessToken == "" {
		return oauthTokenResponse{}, fmt.Errorf("token response missing access token")
	}

	return token, nil
}

func fetchOAuthJSON(accessToken, endpoint string, target any) error {
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("userinfo request failed: %s", string(body))
	}

	return json.NewDecoder(resp.Body).Decode(target)
}

func fetchGithubPrimaryEmail(accessToken string) (string, error) {
	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}

	if err := fetchOAuthJSON(accessToken, "https://api.github.com/user/emails", &emails); err != nil {
		return "", err
	}

	for _, email := range emails {
		if email.Primary && email.Verified {
			return email.Email, nil
		}
	}

	for _, email := range emails {
		if email.Verified {
			return email.Email, nil
		}
	}

	return "", nil
}

// HANDLERS ΓΙΑ GOOGLE
func GoogleLogin(w http.ResponseWriter, r *http.Request) {
	startOAuthLogin(w, r, getGoogleConfig())
}

func GoogleCallback(w http.ResponseWriter, r *http.Request) {
	code := r.FormValue("code")
	if code == "" {
		http.Error(w, "Missing authorization code", http.StatusBadRequest)
		return
	}

	cfg := getGoogleConfig()
	if !validateOAuthState(r, cfg.Name) {
		http.Error(w, "Invalid OAuth state", http.StatusBadRequest)
		return
	}

	token, err := exchangeCodeForToken(cfg, code)
	if err != nil {
		http.Error(w, "Failed to exchange token", http.StatusBadRequest)
		return
	}

	var userInfo struct {
		ID    string `json:"id"`
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := fetchOAuthJSON(token.AccessToken, cfg.UserInfoURL, &userInfo); err != nil {
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
	startOAuthLogin(w, r, getGithubConfig())
}

func GithubCallback(w http.ResponseWriter, r *http.Request) {
	code := r.FormValue("code")
	if code == "" {
		http.Error(w, "Missing authorization code", http.StatusBadRequest)
		return
	}

	cfg := getGithubConfig()
	if !validateOAuthState(r, cfg.Name) {
		http.Error(w, "Invalid OAuth state", http.StatusBadRequest)
		return
	}

	token, err := exchangeCodeForToken(cfg, code)
	if err != nil {
		http.Error(w, "Failed to exchange GitHub token", http.StatusBadRequest)
		return
	}

	var userInfo struct {
		ID    int    `json:"id"`
		Login string `json:"login"`
		Email string `json:"email"`
	}
	if err := fetchOAuthJSON(token.AccessToken, cfg.UserInfoURL, &userInfo); err != nil {
		http.Error(w, "Failed to decode GitHub user info", http.StatusInternalServerError)
		return
	}
	if userInfo.ID == 0 {
		http.Error(w, "Incomplete GitHub user info", http.StatusBadRequest)
		return
	}
	if userInfo.Email == "" {
		email, err := fetchGithubPrimaryEmail(token.AccessToken)
		if err != nil {
			http.Error(w, "Failed to fetch GitHub email", http.StatusInternalServerError)
			return
		}
		userInfo.Email = email
	}
	if userInfo.Email == "" {
		http.Error(w, "Incomplete GitHub user info", http.StatusBadRequest)
		return
	}

	handleOAuthUser(w, r, userInfo.Email, userInfo.Login, strconv.Itoa(userInfo.ID), "github")
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

package main

import (
	"fmt"
	"forum/internal/database"
	"forum/internal/handlers"
	"forum/internal/middleware"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
)

func main() {
	// 1. Έξυπνη φόρτωση του .env
	// Δοκιμάζει πρώτα στο τρέχον folder και μετά στο cmd/
	err := godotenv.Load(".env")
	if err != nil {
		err = godotenv.Load("cmd/.env")
	}

	// Έλεγχος αν φορτώθηκαν τα κλειδιά
	googleID := os.Getenv("GOOGLE_CLIENT_ID")
	if googleID == "" {
		log.Println("⚠️  ΠΡΟΣΟΧΗ: Το .env δεν βρέθηκε ή είναι κενό. Τα OAuth κουμπιά ΔΕΝ θα δουλέψουν.")
	} else {
		fmt.Println("✅ Google ID loaded:", googleID[:10], "...") // Εμφανίζει μόνο την αρχή για ασφάλεια
	}

	// 2. Initialize Database
	log.Println("Initializing Database...")
	database.InitDB()
	defer database.DB.Close()

	// 3. Setup Static Files
	fs := http.FileServer(http.Dir("./ui/static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// 4. Public Routes
	http.HandleFunc("/", handlers.Home)
	http.HandleFunc("/register", handlers.RegisterUser)
	http.HandleFunc("/login", handlers.LoginUser)
	http.HandleFunc("/logout", handlers.LogoutUser)

	// --- OAuth Routes ---
	http.HandleFunc("/login/google", handlers.GoogleLogin)
	http.HandleFunc("/auth/google/callback", handlers.GoogleCallback)
	http.HandleFunc("/login/github", handlers.GithubLogin)
	http.HandleFunc("/auth/github/callback", handlers.GithubCallback)

	// 5. Protected Routes
	http.HandleFunc("/post/create", middleware.RequireLogin(handlers.CreatePost))
	http.HandleFunc("/post", handlers.ViewPost)
	http.HandleFunc("/post/rate", middleware.RequireLogin(handlers.RatePost))
	http.HandleFunc("/comment/create", middleware.RequireLogin(handlers.CreateComment))
	http.HandleFunc("/comment/rate", middleware.RequireLogin(handlers.RateComment))

	// Middleware για Panic Recovery
	finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("Panic recovered: %v", err)
				http.Error(w, "500 - Internal Server Error", http.StatusInternalServerError)
			}
		}()
		http.DefaultServeMux.ServeHTTP(w, r)
	})

	// Start Server
	log.Println("🚀 Server started on http://localhost:8080")
	err = http.ListenAndServe(":8080", finalHandler)
	if err != nil {
		log.Fatal(err)
	}
}

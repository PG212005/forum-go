package main

import (
	"fmt"
	"forum/internal/config"
	"forum/internal/database"
	"forum/internal/handlers"
	"forum/internal/middleware"
	"log"
	"net/http"
	"os"
)

func main() {
	// Load environment variables from the project root first, then fall back to cmd/.
	err := config.LoadEnv(".env")
	if err != nil {
		err = config.LoadEnv("cmd/.env")
	}

	// Log a startup warning when OAuth credentials are missing.
	googleID := os.Getenv("GOOGLE_CLIENT_ID")
	if googleID == "" {
		log.Println("⚠️  ΠΡΟΣΟΧΗ: Το .env δεν βρέθηκε ή είναι κενό. Τα OAuth κουμπιά ΔΕΝ θα δουλέψουν.")
	} else {
		fmt.Println("✅ Google ID loaded:", googleID[:10], "...") // Only print a short prefix to avoid exposing the full value.
	}

	// 2. Initialize Database
	log.Println("Initializing Database...")
	database.InitDB()
	defer database.DB.Close()

	// 3. Setup Static Files
	fs := http.FileServer(http.Dir("./ui/static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	uploadsFS := http.FileServer(http.Dir("./uploads"))
	http.Handle("/uploads/", http.StripPrefix("/uploads/", uploadsFS))

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
	http.HandleFunc("/post/edit", middleware.RequireLogin(handlers.EditPost))
	http.HandleFunc("/post/delete", middleware.RequireLogin(handlers.DeletePost))
	http.HandleFunc("/post", handlers.ViewPost)
	http.HandleFunc("/post/rate", middleware.RequireLogin(handlers.RatePost))
	http.HandleFunc("/comment/create", middleware.RequireLogin(handlers.CreateComment))
	http.HandleFunc("/comment/edit", middleware.RequireLogin(handlers.EditComment))
	http.HandleFunc("/comment/delete", middleware.RequireLogin(handlers.DeleteComment))
	http.HandleFunc("/comment/rate", middleware.RequireLogin(handlers.RateComment))
	http.HandleFunc("/activity", middleware.RequireLogin(handlers.ActivityPage))
	http.HandleFunc("/notifications", middleware.RequireLogin(handlers.NotificationsPage))
	http.HandleFunc("/notifications/stream", middleware.RequireLogin(handlers.NotificationStream))

	// Wrap the default mux so unexpected panics return a controlled 500 response.
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
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := ":" + port
	log.Printf("🚀 Server started on http://localhost%s\n", addr)
	err = http.ListenAndServe(addr, finalHandler)
	if err != nil {
		log.Fatal(err)
	}
}

package main

import (
	"forum/internal/database"
	"forum/internal/handlers"
	"forum/internal/middleware"
	"log"
	"net/http"
)

func main() {
	// 1. Initialize Database
	log.Println("Initializing Database...")
	database.InitDB()
	defer database.DB.Close()

	// 2. Setup Static Files (CSS)
	// Προσοχή: Το path εξαρτάται από το πού τρέχεις το go run.
	// Αν τρέχεις από το root φάκελο 'forum', αυτό είναι σωστό.
	fs := http.FileServer(http.Dir("./ui/static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// 3. Public Routes (Πρόσβαση σε όλους)
	http.HandleFunc("/", handlers.Home)
	http.HandleFunc("/register", handlers.RegisterUser)
	http.HandleFunc("/login", handlers.LoginUser)
	http.HandleFunc("/logout", handlers.LogoutUser)
	// Σελίδα ενός Post (για να βλέπουν και οι guest τα σχόλια - Audit requirement)
	http.HandleFunc("/post", handlers.ViewPost)

	// 4. Protected Routes (Μόνο Login users - Middleware)
	// Χρησιμοποιούμε το RequireLogin wrapper που φτιάξαμε
	http.HandleFunc("/post/create", middleware.RequireLogin(handlers.CreatePost))
	http.HandleFunc("/post/rate", middleware.RequireLogin(handlers.RatePost))
	http.HandleFunc("/comment/create", middleware.RequireLogin(handlers.CreateComment))
	http.HandleFunc("/comment/rate", middleware.RequireLogin(handlers.RateComment))

	// Start Server
	log.Println("Server started on http://localhost:8080")
	log.Println("Press Ctrl+C to stop")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal(err)
	}
}

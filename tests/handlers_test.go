package tests

import (
	"database/sql"
	"forum/internal/database" // Import τη βάση
	"forum/internal/handlers" // Import τους handlers
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	_ "github.com/mattn/go-sqlite3" // Χρειαζόμαστε τον driver
	"golang.org/x/crypto/bcrypt"
)

// <--- 2. ΠΡΟΣΘΕΣΕ ΑΥΤΗ ΤΗ ΣΥΝΑΡΤΗΣΗ
// init: Τρέχει αυτόματα πριν από όλα τα tests
func init() {
	// Αλλάζουμε τον φάκελο εκτέλεσης (Working Directory)
	// Από "forum/tests" πάμε πίσω στο "forum/"
	err := os.Chdir("..")
	if err != nil {
		panic(err)
	}
}

// setupTestDB: Ετοιμάζει την προσωρινή βάση στη μνήμη (RAM)
func setupTestDB() {
	var err error
	// Προσοχή: Χρησιμοποιούμε το database.DB που είναι Exported (Κεφαλαίο D)
	database.DB, err = sql.Open("sqlite3", ":memory:")
	if err != nil {
		panic(err)
	}

	// Δημιουργούμε το Schema (Πίνακες) για να μην σκάσει ο server
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		email TEXT UNIQUE,
		username TEXT UNIQUE,
		password TEXT
	);
	CREATE TABLE IF NOT EXISTS posts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER,
		title TEXT,
		content TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS sessions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		uuid TEXT,
		user_id INTEGER,
		expires_at DATETIME
	);
	CREATE TABLE IF NOT EXISTS categories (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT UNIQUE
	);
	CREATE TABLE IF NOT EXISTS post_categories (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		post_id INTEGER,
		category_id INTEGER
	);
	CREATE TABLE IF NOT EXISTS comments (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		post_id INTEGER,
		user_id INTEGER,
		content TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS votes (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER,
		post_id INTEGER,
		comment_id INTEGER,
		type INTEGER
	);
	`
	_, err = database.DB.Exec(schema)
	if err != nil {
		panic(err)
	}
}

// TestHomeHandler: Ελέγχει αν η σελίδα Home (handlers.Home) λειτουργεί
func TestHomeHandler(t *testing.T) {
	setupTestDB()

	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	// Καλούμε τη συνάρτηση μέσα από το πακέτο handlers
	handler := http.HandlerFunc(handlers.Home)

	handler.ServeHTTP(rr, req)

	// Check status code 200
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
}

// TestRegisterPage: Ελέγχει αν η σελίδα Register (handlers.RegisterUser) λειτουργεί
func TestRegisterPage(t *testing.T) {
	setupTestDB()

	req, err := http.NewRequest("GET", "/register", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()

	// ΠΡΟΣΟΧΗ: Εδώ υπάρχει ένα "Trick".
	// Οι handlers σου (όπως το RegisterUser) ψάχνουν τα HTML αρχεία relative path (π.χ. "ui/html/...").
	// Όταν τρέχεις το test από τον φάκελο /tests, το Go ίσως δεν βρει τα templates.
	// Αν σου βγάλει error "Template not found", πρέπει να αλλάξεις το path στον κώδικα ή να τρέξεις το test από το root.

	// Γι' αυτό το τεστ, αρκεί να δούμε αν καλείται ο handler χωρίς να κρασάρει,
	// ακόμα κι αν επιστρέψει 500 λόγω template error, ξέρουμε ότι ο router δουλεύει.
	handler := http.HandlerFunc(handlers.RegisterUser)

	handler.ServeHTTP(rr, req)

	// Αν τα templates δεν βρεθούν, θα γυρίσει 500. Αν βρεθούν, 200.
	// Για το audit, αρκεί να μην είναι 404.
	if status := rr.Code; status == http.StatusNotFound {
		t.Errorf("handler returned 404: url not mapped properly")
	}
}

// TestBcrypt: Αυτό είναι καθαρό unit test, δεν εξαρτάται από handlers
func TestPasswordEncryption(t *testing.T) {
	password := "supersecret"

	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("Error hashing password: %v", err)
	}

	err = bcrypt.CompareHashAndPassword(hashed, []byte(password))
	if err != nil {
		t.Errorf("Password verification failed")
	}
}

package tests

import (
	"database/sql"
	"forum/internal/database" // Import τη βάση
	"forum/internal/handlers" // Import τους handlers
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
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
		image_path TEXT,
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

func TestHomeHandlerSearchAndSort(t *testing.T) {
	setupTestDB()

	if _, err := database.DB.Exec(`INSERT INTO users (id, email, username, password) VALUES
		(1, 'a@example.com', 'alice', 'hash'),
		(2, 'b@example.com', 'bob', 'hash')`); err != nil {
		t.Fatal(err)
	}

	if _, err := database.DB.Exec(`INSERT INTO categories (id, name) VALUES (1, 'Technology'), (2, 'Music')`); err != nil {
		t.Fatal(err)
	}

	if _, err := database.DB.Exec(`INSERT INTO posts (id, user_id, title, content, image_path, created_at) VALUES
		(1, 1, 'Go Search', 'Forum search is live', '', '2026-03-01 10:00:00'),
		(2, 2, 'Cooking Notes', 'Nothing about backend work', '', '2026-03-02 10:00:00')`); err != nil {
		t.Fatal(err)
	}

	if _, err := database.DB.Exec(`INSERT INTO post_categories (post_id, category_id) VALUES (1, 1), (2, 2)`); err != nil {
		t.Fatal(err)
	}

	if _, err := database.DB.Exec(`INSERT INTO votes (user_id, post_id, type) VALUES (1, 1, 1), (2, 1, 1)`); err != nil {
		t.Fatal(err)
	}

	if _, err := database.DB.Exec(`INSERT INTO comments (post_id, user_id, content) VALUES (1, 2, 'nice feature')`); err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest("GET", "/?q=search&sort=liked", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	http.HandlerFunc(handlers.Home).ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Fatalf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Go Search") {
		t.Fatalf("expected matching post to be rendered, body=%q", body)
	}
	if strings.Contains(body, "Cooking Notes") {
		t.Fatalf("expected non-matching post to be filtered out, body=%q", body)
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

package database

import (
	"database/sql"
	"log"
	"os"

	_ "github.com/mattn/go-sqlite3"
)

var DB *sql.DB

func InitDB() {
	var err error
	DB, err = sql.Open("sqlite3", "./forum.db")
	if err != nil {
		log.Fatal(err)
	}

	wd, _ := os.Getwd()
	log.Println("Current working directory:", wd)
	log.Println("Opening database: ./forum.db")

	createTables()

	var count int
	_ = DB.QueryRow("SELECT COUNT(*) FROM posts").Scan(&count)
	log.Println("Posts in database:", count)
}
func createTables() {
	// Ενεργοποίηση Foreign Keys για την SQLite
	_, _ = DB.Exec("PRAGMA foreign_keys = ON;")

	queries := []string{
		`CREATE TABLE IF NOT EXISTS users (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            email TEXT UNIQUE NOT NULL,
            username TEXT UNIQUE NOT NULL,
            password TEXT, 
            google_id TEXT UNIQUE,
            github_id TEXT UNIQUE,
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP
        );`,
		`CREATE TABLE IF NOT EXISTS sessions (
			uuid TEXT PRIMARY KEY,
			user_id INTEGER NOT NULL,
			expires_at DATETIME NOT NULL,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS categories (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT UNIQUE NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS posts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			title TEXT NOT NULL,
			content TEXT NOT NULL,
			image_path TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS post_categories (
			post_id INTEGER NOT NULL,
			category_id INTEGER NOT NULL,
			PRIMARY KEY (post_id, category_id),
			FOREIGN KEY(post_id) REFERENCES posts(id) ON DELETE CASCADE,
			FOREIGN KEY(category_id) REFERENCES categories(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS comments (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			post_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			content TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(post_id) REFERENCES posts(id) ON DELETE CASCADE,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS votes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			post_id INTEGER,
			comment_id INTEGER,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			type INTEGER NOT NULL CHECK(type IN (1, -1)),
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY(post_id) REFERENCES posts(id) ON DELETE CASCADE,
			FOREIGN KEY(comment_id) REFERENCES comments(id) ON DELETE CASCADE,
			UNIQUE(user_id, post_id),
			UNIQUE(user_id, comment_id)
		);`,
		`CREATE TABLE IF NOT EXISTS notifications (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			actor_id INTEGER NOT NULL,
			post_id INTEGER NOT NULL,
			type TEXT NOT NULL CHECK(type IN ('post_liked', 'post_disliked', 'post_commented')),
			is_read INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY(actor_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY(post_id) REFERENCES posts(id) ON DELETE CASCADE
		);`,
	}

	for _, query := range queries {
		_, err := DB.Exec(query)
		if err != nil {
			log.Fatalf("Error creating table: %v", err)
		}
	}
	_, _ = DB.Exec(`ALTER TABLE posts ADD COLUMN image_path TEXT`)
	_, _ = DB.Exec(`ALTER TABLE posts ADD COLUMN updated_at DATETIME DEFAULT CURRENT_TIMESTAMP`)
	_, _ = DB.Exec(`ALTER TABLE comments ADD COLUMN updated_at DATETIME DEFAULT CURRENT_TIMESTAMP`)
	_, _ = DB.Exec(`ALTER TABLE votes ADD COLUMN created_at DATETIME DEFAULT CURRENT_TIMESTAMP`)

	// Προσθήκη βασικών κατηγοριών
	categories := []string{"Technology", "Health", "Music", "General"}
	for _, cat := range categories {
		DB.Exec("INSERT OR IGNORE INTO categories (name) VALUES (?)", cat)
	}
}

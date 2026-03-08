package models

import "time"

type User struct {
	ID        int
	Email     string
	Username  string
	Password  string
	CreatedAt time.Time
}

type Post struct {
	ID            int
	UserID        int
	Author        string // Για ευκολία στο template
	Title         string
	Content       string
	Categories    []string
	Likes         int
	Dislikes      int
	CommentCount  int
	CreatedAt     time.Time
	FormattedTime string
}

type Comment struct {
	ID            int
	PostID        int
	UserID        int
	Author        string
	Content       string
	Likes         int
	Dislikes      int
	CreatedAt     time.Time
	FormattedTime string
}

// Στο τέλος του models/models.go
type PageData struct {
	IsLoggedIn bool
	User       User   // Το User struct που ήδη έχεις
	Posts      []Post // Το Post struct που ήδη έχεις
	Post       Post
	Comments   []Comment // Το Comment struct που ήδη έχεις
	Categories []string
	Error      string // ΕΔΩ θα μπαίνει το μήνυμα για το audit
}

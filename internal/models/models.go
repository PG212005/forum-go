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

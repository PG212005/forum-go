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
	ImagePath     string
	Categories    []string
	Likes         int
	Dislikes      int
	CommentCount  int
	CreatedAt     time.Time
	UpdatedAt     time.Time
	FormattedTime string
	CanManage     bool
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
	UpdatedAt     time.Time
	FormattedTime string
	PostTitle     string
	PostAuthor    string
	CanManage     bool
}

type Notification struct {
	ID        int
	ActorID   int
	ActorName string
	UserID    int
	PostID    int
	PostTitle string
	Type      string
	IsRead    bool
	CreatedAt time.Time
}

type ActivityVote struct {
	ID            int
	Type          int
	TargetType    string
	TargetID      int
	TargetContent string
	PostID        int
	PostTitle     string
	CreatedAt     time.Time
}

type PageData struct {
	IsLoggedIn         bool
	User               User
	Posts              []Post
	Post               Post
	Comments           []Comment
	Categories         []string
	Notifications      []Notification
	UnreadNotifications int
	ActivityPosts      []Post
	ActivityVotes      []ActivityVote
	ActivityComments   []Comment
	ManagedComments    []Comment
	FormAction         string
	SubmitLabel        string
	PageTitle          string
	SelectedCategories map[string]bool
	Success            string
	Error              string
}

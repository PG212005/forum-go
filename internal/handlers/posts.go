package handlers

import (
	"database/sql"
	"forum/internal/database"
	"forum/internal/models"
	"html/template"
	"net/http"
	"strings"
)

// PageData: Δομή για να στέλνουμε δεδομένα στο HTML
type PageData struct {
	User       models.User
	IsLoggedIn bool
	Posts      []models.Post
	Post       models.Post      // Για single post view
	Comments   []models.Comment // Για comments
	Categories []string         // Για τα φίλτρα και τη δημιουργία
	Filter     string           // Ποιο φίλτρο είναι ενεργό
	Error      string
}

// Home handles the main page and filters
func Home(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	user, isLoggedIn := GetUserFromSession(r)

	// Λήψη παραμέτρων φίλτρων
	filterBy := r.URL.Query().Get("filter")     // liked, created
	categoryBy := r.URL.Query().Get("category") // tech, health...

	var rows *sql.Rows
	var err error

	// Δυναμικό SQL Query ανάλογα με τα φίλτρα (AUDIT CRITICAL)
	baseQuery := `SELECT p.id, p.title, p.content, u.username, p.created_at,
				  (SELECT COUNT(*) FROM votes WHERE post_id = p.id AND type = 1) as likes,
				  (SELECT COUNT(*) FROM votes WHERE post_id = p.id AND type = -1) as dislikes
				  FROM posts p
				  JOIN users u ON p.user_id = u.id`

	if categoryBy != "" {
		// Φίλτρο Κατηγορίας
		query := baseQuery + ` JOIN post_categories pc ON p.id = pc.post_id 
							   JOIN categories c ON pc.category_id = c.id 
							   WHERE c.name = ? ORDER BY p.created_at DESC`
		rows, err = database.DB.Query(query, categoryBy)
	} else if filterBy == "liked" && isLoggedIn {
		// Φίλτρο "Liked Posts"
		query := baseQuery + ` JOIN votes v ON p.id = v.post_id 
							   WHERE v.user_id = ? AND v.type = 1 ORDER BY p.created_at DESC`
		rows, err = database.DB.Query(query, user.ID)
	} else if filterBy == "created" && isLoggedIn {
		// Φίλτρο "My Posts"
		query := baseQuery + ` WHERE p.user_id = ? ORDER BY p.created_at DESC`
		rows, err = database.DB.Query(query, user.ID)
	} else {
		// Default: Όλα τα posts
		query := baseQuery + ` ORDER BY p.created_at DESC`
		rows, err = database.DB.Query(query)
	}

	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var posts []models.Post
	for rows.Next() {
		var p models.Post
		rows.Scan(&p.ID, &p.Title, &p.Content, &p.Author, &p.CreatedAt, &p.Likes, &p.Dislikes)
		// Fetch categories for this post
		catRows, _ := database.DB.Query("SELECT c.name FROM categories c JOIN post_categories pc ON c.id = pc.category_id WHERE pc.post_id = ?", p.ID)
		for catRows.Next() {
			var cName string
			catRows.Scan(&cName)
			p.Categories = append(p.Categories, cName)
		}
		posts = append(posts, p)
	}

	// Fetch all categories for sidebar
	catRows, _ := database.DB.Query("SELECT name FROM categories")
	var allCats []string
	for catRows.Next() {
		var cName string
		catRows.Scan(&cName)
		allCats = append(allCats, cName)
	}

	data := PageData{
		User:       user,
		IsLoggedIn: isLoggedIn,
		Posts:      posts,
		Categories: allCats,
	}

	tmpl, _ := template.ParseFiles("ui/html/base.layout.html", "ui/html/home.page.html")
	tmpl.Execute(w, data)
}

// CreatePost handles creating a new post
func CreatePost(w http.ResponseWriter, r *http.Request) {
	user, isLoggedIn := GetUserFromSession(r)
	if !isLoggedIn {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if r.Method == http.MethodPost {
		title := r.FormValue("title")
		content := r.FormValue("content")
		categories := r.Form["categories"] // List of selected categories

		// Audit check: Empty post
		if strings.TrimSpace(title) == "" || strings.TrimSpace(content) == "" {
			http.Error(w, "Title and Content cannot be empty", http.StatusBadRequest)
			return
		}
		if len(categories) == 0 {
			http.Error(w, "At least one category is required", http.StatusBadRequest)
			return
		}

		res, err := database.DB.Exec("INSERT INTO posts (user_id, title, content) VALUES (?, ?, ?)", user.ID, title, content)
		if err != nil {
			http.Error(w, "Server error", http.StatusInternalServerError)
			return
		}

		postID, _ := res.LastInsertId()

		// Link categories
		for _, catName := range categories {
			var catID int
			database.DB.QueryRow("SELECT id FROM categories WHERE name = ?", catName).Scan(&catID)
			database.DB.Exec("INSERT INTO post_categories (post_id, category_id) VALUES (?, ?)", postID, catID)
		}

		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// GET Request: Show Form
	catRows, _ := database.DB.Query("SELECT name FROM categories")
	var allCats []string
	for catRows.Next() {
		var cName string
		catRows.Scan(&cName)
		allCats = append(allCats, cName)
	}

	data := PageData{User: user, IsLoggedIn: true, Categories: allCats}
	tmpl, _ := template.ParseFiles("ui/html/base.layout.html", "ui/html/create.page.html")
	tmpl.Execute(w, data)
}

// RatePost handles Likes/Dislikes logic
func RatePost(w http.ResponseWriter, r *http.Request) {
	user, isLoggedIn := GetUserFromSession(r)
	if !isLoggedIn {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	postID := r.FormValue("post_id")
	action := r.FormValue("action") // "like" or "dislike"
	voteType := 1
	if action == "dislike" {
		voteType = -1
	}

	// 1. Check existing vote
	var existingType int
	err := database.DB.QueryRow("SELECT type FROM votes WHERE user_id = ? AND post_id = ?", user.ID, postID).Scan(&existingType)

	if err == sql.ErrNoRows {
		// New Vote
		database.DB.Exec("INSERT INTO votes (user_id, post_id, type) VALUES (?, ?, ?)", user.ID, postID, voteType)
	} else {
		// Existing Vote logic (Toggle or Change)
		if existingType == voteType {
			// Clicked same button -> Remove vote (Toggle off)
			database.DB.Exec("DELETE FROM votes WHERE user_id = ? AND post_id = ?", user.ID, postID)
		} else {
			// Clicked different button -> Update vote
			database.DB.Exec("UPDATE votes SET type = ? WHERE user_id = ? AND post_id = ?", voteType, user.ID, postID)
		}
	}

	// Redirect back to where they came from
	http.Redirect(w, r, r.Header.Get("Referer"), http.StatusSeeOther)
}

// ViewPost handles displaying a single post and its comments
func ViewPost(w http.ResponseWriter, r *http.Request) {
	// Παίρνουμε το ID από το URL (π.χ. /post?id=5)
	postID := r.URL.Query().Get("id")
	if postID == "" {
		http.Error(w, "Missing post ID", http.StatusBadRequest)
		return
	}

	user, isLoggedIn := GetUserFromSession(r)

	// 1. Fetch Post Details
	// Χρησιμοποιούμε το ίδιο query με το Home αλλά για συγκεκριμένο ID
	query := `SELECT p.id, p.title, p.content, u.username, p.created_at,
              (SELECT COUNT(*) FROM votes WHERE post_id = p.id AND type = 1) as likes,
              (SELECT COUNT(*) FROM votes WHERE post_id = p.id AND type = -1) as dislikes
              FROM posts p
              JOIN users u ON p.user_id = u.id
              WHERE p.id = ?`

	var p models.Post
	err := database.DB.QueryRow(query, postID).Scan(&p.ID, &p.Title, &p.Content, &p.Author, &p.CreatedAt, &p.Likes, &p.Dislikes)
	if err == sql.ErrNoRows {
		http.NotFound(w, r)
		return
	} else if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Fetch categories for this post
	catRows, _ := database.DB.Query("SELECT c.name FROM categories c JOIN post_categories pc ON c.id = pc.category_id WHERE pc.post_id = ?", p.ID)
	for catRows.Next() {
		var cName string
		catRows.Scan(&cName)
		p.Categories = append(p.Categories, cName)
	}

	// 2. Fetch Comments for this Post
	// Εμφανίζουμε και τα Likes/Dislikes των σχολίων (Audit Requirement)
	commentQuery := `SELECT c.id, c.content, u.username, c.created_at,
                     (SELECT COUNT(*) FROM votes WHERE comment_id = c.id AND type = 1) as likes,
                     (SELECT COUNT(*) FROM votes WHERE comment_id = c.id AND type = -1) as dislikes
                     FROM comments c
                     JOIN users u ON c.user_id = u.id
                     WHERE c.post_id = ?
                     ORDER BY c.created_at ASC`

	rows, err := database.DB.Query(commentQuery, postID)
	if err != nil {
		http.Error(w, "Database error fetching comments", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var comments []models.Comment
	for rows.Next() {
		var c models.Comment
		rows.Scan(&c.ID, &c.Content, &c.Author, &c.CreatedAt, &c.Likes, &c.Dislikes)
		comments = append(comments, c)
	}

	data := PageData{
		User:       user,
		IsLoggedIn: isLoggedIn,
		Post:       p,        // Το συγκεκριμένο post
		Comments:   comments, // Τα σχόλιά του
	}

	tmpl, err := template.ParseFiles("ui/html/base.layout.html", "ui/html/post.page.html")
	if err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, data)
}

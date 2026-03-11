package handlers

import (
	"database/sql"
	"fmt"
	"forum/internal/database"
	"forum/internal/models"
	"html/template"
	"log"
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
		ErrorPage(w, http.StatusNotFound, "404 - Page Not Found")
		return
	}

	user, isLoggedIn := GetUserFromSession(r)

	// Λήψη παραμέτρων φίλτρων
	filterBy := r.URL.Query().Get("filter")     // liked, created
	categoryBy := r.URL.Query().Get("category") // tech, health...

	// Προετοιμασία sidebar κατηγοριών (για να εμφανίζονται πάντα)
	var allCats []string
	catRows, err := database.DB.Query("SELECT name FROM categories")
	if err == nil {
		for catRows.Next() {
			var cName string
			catRows.Scan(&cName)
			allCats = append(allCats, cName)
		}
		catRows.Close()
	}

	if (filterBy == "liked" || filterBy == "created") && !isLoggedIn {
		data := PageData{
			IsLoggedIn: false,
			Categories: allCats,
			Error:      "You must be logged in to view your personalized filters.",
		}
		tmpl, _ := template.ParseFiles("ui/html/base.layout.html", "ui/html/home.page.html")
		tmpl.Execute(w, data)
		return
	}

	var rows *sql.Rows

	// Δυναμικό SQL Query ανάλογα με τα φίλτρα
	baseQuery := `SELECT p.id, p.title, p.content, u.username, p.created_at,
              (SELECT COUNT(*) FROM votes WHERE post_id = p.id AND type = 1) as likes,
              (SELECT COUNT(*) FROM votes WHERE post_id = p.id AND type = -1) as dislikes,
              (SELECT COUNT(*) FROM comments WHERE post_id = p.id) as comment_count
              FROM posts p
              JOIN users u ON p.user_id = u.id`

	if categoryBy != "" {
		query := baseQuery + ` JOIN post_categories pc ON p.id = pc.post_id 
                               JOIN categories c ON pc.category_id = c.id 
                               WHERE c.name = ? ORDER BY p.created_at DESC`
		rows, err = database.DB.Query(query, categoryBy)
	} else if filterBy == "liked" && isLoggedIn {
		query := baseQuery + ` JOIN votes v ON p.id = v.post_id 
                               WHERE v.user_id = ? AND v.type = 1 ORDER BY p.created_at DESC`
		rows, err = database.DB.Query(query, user.ID)
	} else if filterBy == "created" && isLoggedIn {
		query := baseQuery + ` WHERE p.user_id = ? ORDER BY p.created_at DESC`
		rows, err = database.DB.Query(query, user.ID)
	} else {
		query := baseQuery + ` ORDER BY p.created_at DESC`
		rows, err = database.DB.Query(query)
	}

	if err != nil {
		log.Printf("Database Error in Home: %v", err)
		ErrorPage(w, http.StatusInternalServerError, "500 - Internal Server Error")
		return
	}
	defer rows.Close()

	var posts []models.Post
	for rows.Next() {
		var p models.Post
		err := rows.Scan(&p.ID, &p.Title, &p.Content, &p.Author, &p.CreatedAt, &p.Likes, &p.Dislikes, &p.CommentCount)
		if err != nil {
			log.Printf("Scan error in Home: %v", err)
			continue
		}

		// ΔΙΟΡΘΩΜΕΝΟ QUERY ΚΑΤΗΓΟΡΙΩΝ (Χωρίς το typo)
		postCatRows, err := database.DB.Query("SELECT c.name FROM categories c JOIN post_categories pc ON c.id = pc.category_id WHERE pc.post_id = ?", p.ID)
		if err == nil {
			for postCatRows.Next() {
				var cName string
				postCatRows.Scan(&cName)
				p.Categories = append(p.Categories, cName)
			}
			postCatRows.Close()
		}
		posts = append(posts, p)
	}

	data := PageData{
		User:       user,
		IsLoggedIn: isLoggedIn,
		Posts:      posts,
		Categories: allCats,
		Filter:     filterBy,
	}

	tmpl, err := template.ParseFiles("ui/html/base.layout.html", "ui/html/home.page.html")
	if err != nil {
		log.Printf("Template Error in Home: %v", err)
		ErrorPage(w, http.StatusInternalServerError, "500 - Template Error")
		return
	}
	tmpl.Execute(w, data)
}

// CreatePost handles creating a new post
func CreatePost(w http.ResponseWriter, r *http.Request) {
	user, isLoggedIn := GetUserFromSession(r)
	if !isLoggedIn {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	var allCats []string
	catRows, err := database.DB.Query("SELECT name FROM categories")
	if err == nil {
		for catRows.Next() {
			var cName string
			catRows.Scan(&cName)
			allCats = append(allCats, cName)
		}
		catRows.Close()
	}

	if r.Method == http.MethodPost {
		title := r.FormValue("title")
		content := r.FormValue("content")
		categories := r.Form["categories"]

		var errMsg string
		if strings.TrimSpace(title) == "" || strings.TrimSpace(content) == "" {
			errMsg = "Title and Content cannot be empty"
		} else if len(categories) == 0 {
			errMsg = "At least one category is required"
		}

		if errMsg != "" {
			data := PageData{
				User:       user,
				IsLoggedIn: true,
				Categories: allCats,
				Error:      errMsg,
				Post: models.Post{
					Title:   title,
					Content: content,
				},
			}
			tmpl, _ := template.ParseFiles("ui/html/base.layout.html", "ui/html/create.page.html")
			tmpl.Execute(w, data)
			return
		}

		res, err := database.DB.Exec("INSERT INTO posts (user_id, title, content) VALUES (?, ?, ?)", user.ID, title, content)
		if err != nil {
			log.Printf("Insert Post Error: %v", err)
			ErrorPage(w, http.StatusInternalServerError, "500 - Failed to create post")
			return
		}

		postID, _ := res.LastInsertId()

		for _, catName := range categories {
			var catID int
			err := database.DB.QueryRow("SELECT id FROM categories WHERE name = ?", catName).Scan(&catID)
			if err == nil {
				database.DB.Exec("INSERT INTO post_categories (post_id, category_id) VALUES (?, ?)", postID, catID)
			}
		}

		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
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
	action := r.FormValue("action")
	voteType := 1
	if action == "dislike" {
		voteType = -1
	}

	var existingType int
	err := database.DB.QueryRow("SELECT type FROM votes WHERE user_id = ? AND post_id = ?", user.ID, postID).Scan(&existingType)

	if err == sql.ErrNoRows {
		database.DB.Exec("INSERT INTO votes (user_id, post_id, type) VALUES (?, ?, ?)", user.ID, postID, voteType)
	} else {
		if existingType == voteType {
			database.DB.Exec("DELETE FROM votes WHERE user_id = ? AND post_id = ?", user.ID, postID)
		} else {
			database.DB.Exec("UPDATE votes SET type = ? WHERE user_id = ? AND post_id = ?", voteType, user.ID, postID)
		}
	}

	referer := r.Header.Get("Referer")
	target := referer
	if !strings.Contains(referer, "/post?id=") {
		target = fmt.Sprintf("/#post-%s", postID)
	}
	if target == "" {
		target = "/"
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

// ViewPost handles displaying a single post and its comments
func ViewPost(w http.ResponseWriter, r *http.Request) {
	postID := r.URL.Query().Get("id")
	if postID == "" {
		ErrorPage(w, http.StatusBadRequest, "400 - Bad Request: Missing Post ID")
		return
	}

	user, isLoggedIn := GetUserFromSession(r)

	query := `SELECT p.id, p.title, p.content, u.username, p.created_at,
              (SELECT COUNT(*) FROM votes WHERE post_id = p.id AND type = 1) as likes,
              (SELECT COUNT(*) FROM votes WHERE post_id = p.id AND type = -1) as dislikes
              FROM posts p
              JOIN users u ON p.user_id = u.id
              WHERE p.id = ?`

	var p models.Post
	err := database.DB.QueryRow(query, postID).Scan(&p.ID, &p.Title, &p.Content, &p.Author, &p.CreatedAt, &p.Likes, &p.Dislikes)

	if err == sql.ErrNoRows {
		ErrorPage(w, http.StatusNotFound, "404 - Post Not Found")
		return
	} else if err != nil {
		log.Printf("Database error in ViewPost: %v", err)
		ErrorPage(w, http.StatusInternalServerError, "500 - Internal Server Error")
		return
	}

	// Ασφαλής ανάκτηση κατηγοριών
	catRows, err := database.DB.Query("SELECT c.name FROM categories c JOIN post_categories pc ON c.id = pc.category_id WHERE pc.post_id = ?", p.ID)
	if err == nil {
		defer catRows.Close()
		for catRows.Next() {
			var cName string
			catRows.Scan(&cName)
			p.Categories = append(p.Categories, cName)
		}
	}

	commentQuery := `SELECT c.id, c.content, u.username, c.created_at,
                     (SELECT COUNT(*) FROM votes WHERE comment_id = c.id AND type = 1) as likes,
                     (SELECT COUNT(*) FROM votes WHERE comment_id = c.id AND type = -1) as dislikes
                     FROM comments c
                     JOIN users u ON c.user_id = u.id
                     WHERE c.post_id = ?
                     ORDER BY c.created_at ASC`

	rows, err := database.DB.Query(commentQuery, postID)
	if err != nil {
		log.Printf("Comments Fetch Error: %v", err)
		ErrorPage(w, http.StatusInternalServerError, "500 - Error loading comments")
		return
	}
	defer rows.Close()

	var comments []models.Comment
	for rows.Next() {
		var c models.Comment
		err := rows.Scan(&c.ID, &c.Content, &c.Author, &c.CreatedAt, &c.Likes, &c.Dislikes)
		if err == nil {
			comments = append(comments, c)
		}
	}

	data := PageData{
		User:       user,
		IsLoggedIn: isLoggedIn,
		Post:       p,
		Comments:   comments,
	}

	tmpl, err := template.ParseFiles("ui/html/base.layout.html", "ui/html/post.page.html")
	if err != nil {
		log.Printf("Template Error in ViewPost: %v", err)
		ErrorPage(w, http.StatusInternalServerError, "500 - Template Error")
		return
	}
	tmpl.Execute(w, data)
}

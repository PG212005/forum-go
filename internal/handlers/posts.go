package handlers

import (
	"database/sql"
	"errors"
	"fmt"
	"forum/internal/database"
	"forum/internal/models"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gofrs/uuid"
)

// PageData contains the template data shared across the post-related pages.
type PageData struct {
	User                models.User
	IsLoggedIn          bool
	Posts               []models.Post
	Post                models.Post
	Comments            []models.Comment
	Categories          []string
	Notifications       []models.Notification
	UnreadNotifications int
	ActivityPosts       []models.Post
	ActivityVotes       []models.ActivityVote
	ActivityComments    []models.Comment
	ManagedComments     []models.Comment
	Filter              string
	Category            string
	SearchQuery         string
	SortBy              string
	FormAction          string
	SubmitLabel         string
	PageTitle           string
	SelectedCategories  map[string]bool
	Success             string
	Error               string
}

type homeFilters struct {
	filter   string
	category string
	search   string
	sort     string
}

func parseHomeFilters(r *http.Request) homeFilters {
	return homeFilters{
		filter:   r.URL.Query().Get("filter"),
		category: r.URL.Query().Get("category"),
		search:   strings.TrimSpace(r.URL.Query().Get("q")),
		sort:     normalizeSort(r.URL.Query().Get("sort")),
	}
}

// Home handles the main page and filters
func Home(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		ErrorPage(w, http.StatusNotFound, "404 - Page Not Found")
		return
	}

	user, isLoggedIn := GetUserFromSession(r)
	filters := parseHomeFilters(r)
	allCats := loadCategories()

	if requiresLogin(filters) && !isLoggedIn {
		data := PageData{
			User:        user,
			IsLoggedIn:  false,
			Categories:  allCats,
			Category:    filters.category,
			SearchQuery: filters.search,
			SortBy:      filters.sort,
			Error:       "You must be logged in to view your personalized filters.",
		}
		_ = renderTemplate(w, "ui/html/home.page.html", data)
		return
	}

	query, args := buildHomeQuery(filters, user.ID, isLoggedIn)
	posts, err := queryPosts(query, args, user.ID, isLoggedIn)
	if err != nil {
		log.Printf("Database Error in Home: %v", err)
		ErrorPage(w, http.StatusInternalServerError, "500 - Internal Server Error")
		return
	}

	data := PageData{
		User:                user,
		IsLoggedIn:          isLoggedIn,
		UnreadNotifications: getUnreadNotificationCount(user.ID),
		Posts:               posts,
		Categories:          allCats,
		Filter:              filters.filter,
		Category:            filters.category,
		SearchQuery:         filters.search,
		SortBy:              filters.sort,
	}

	if err := renderTemplate(w, "ui/html/home.page.html", data); err != nil {
		log.Printf("Template Error in Home: %v", err)
		ErrorPage(w, http.StatusInternalServerError, "500 - Template Error")
	}
}

func requiresLogin(filters homeFilters) bool {
	return filters.filter == "liked" || filters.filter == "created"
}

func buildHomeQuery(filters homeFilters, userID int, isLoggedIn bool) (string, []any) {
	query := `SELECT p.id, p.user_id, p.title, p.content, p.image_path, u.username, p.created_at,
              (SELECT COUNT(*) FROM votes WHERE post_id = p.id AND type = 1) as likes,
              (SELECT COUNT(*) FROM votes WHERE post_id = p.id AND type = -1) as dislikes,
              (SELECT COUNT(*) FROM comments WHERE post_id = p.id) as comment_count,
              COALESCE(GROUP_CONCAT(DISTINCT c.name), '') as categories
              FROM posts p
              JOIN users u ON p.user_id = u.id
              LEFT JOIN post_categories pcg ON pcg.post_id = p.id
              LEFT JOIN categories c ON c.id = pcg.category_id`
	var (
		conditions []string
		args       []any
	)

	if filters.category != "" {
		conditions = append(conditions, `EXISTS (
			SELECT 1
			FROM post_categories pc
			JOIN categories c ON pc.category_id = c.id
			WHERE pc.post_id = p.id AND c.name = ?
		)`)
		args = append(args, filters.category)
	}

	if filters.filter == "liked" && isLoggedIn {
		conditions = append(conditions, `EXISTS (
			SELECT 1
			FROM votes v
			WHERE v.post_id = p.id AND v.user_id = ? AND v.type = 1
		)`)
		args = append(args, userID)
	} else if filters.filter == "created" && isLoggedIn {
		conditions = append(conditions, "p.user_id = ?")
		args = append(args, userID)
	}

	if filters.search != "" {
		pattern := "%" + filters.search + "%"
		conditions = append(conditions, `(LOWER(p.title) LIKE LOWER(?)
			OR LOWER(p.content) LIKE LOWER(?)
			OR LOWER(u.username) LIKE LOWER(?)
			OR EXISTS (
				SELECT 1
				FROM post_categories spc
				JOIN categories sc ON spc.category_id = sc.id
				WHERE spc.post_id = p.id AND LOWER(sc.name) LIKE LOWER(?)
			))`)
		args = append(args, pattern, pattern, pattern, pattern)
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " GROUP BY p.id, p.user_id, p.title, p.content, p.image_path, u.username, p.created_at"
	query += " " + orderByClause(filters.sort)
	return query, args
}

func queryPosts(query string, args []any, userID int, isLoggedIn bool) ([]models.Post, error) {
	rows, err := database.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []models.Post
	for rows.Next() {
		p, err := scanPostSummary(rows, userID, isLoggedIn)
		if err != nil {
			log.Printf("Scan error in Home: %v", err)
			continue
		}
		posts = append(posts, p)
	}
	return posts, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanPostSummary(row rowScanner, userID int, isLoggedIn bool) (models.Post, error) {
	var p models.Post
	var imagePath sql.NullString
	var categoryList string
	err := row.Scan(
		&p.ID, &p.UserID, &p.Title, &p.Content, &imagePath, &p.Author,
		&p.CreatedAt, &p.Likes, &p.Dislikes, &p.CommentCount, &categoryList,
	)
	if err != nil {
		return models.Post{}, err
	}
	if imagePath.Valid {
		p.ImagePath = imagePath.String
	}
	if categoryList != "" {
		p.Categories = strings.Split(categoryList, ",")
	}
	p.CanManage = isLoggedIn && p.UserID == userID
	return p, nil
}

// normalizeSort whitelists supported sort values and falls back to "newest".
func normalizeSort(raw string) string {
	switch raw {
	case "oldest", "liked", "discussed":
		return raw
	default:
		return "newest"
	}
}

// orderByClause returns the SQL ORDER BY clause for the selected post sort mode.
func orderByClause(sortBy string) string {
	switch sortBy {
	case "oldest":
		return "ORDER BY p.created_at ASC"
	case "liked":
		return "ORDER BY likes DESC, p.created_at DESC"
	case "discussed":
		return "ORDER BY comment_count DESC, p.created_at DESC"
	default:
		return "ORDER BY p.created_at DESC"
	}
}

const maxUploadSize = 20 << 20 // 20 MB

// saveUploadedImage validates content type, persists the file under /uploads,
// and returns the public URL path.
func saveUploadedImage(file multipart.File, header *multipart.FileHeader) (string, error) {
	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return "", err
	}

	contentType := http.DetectContentType(buffer[:n])
	allowedTypes := map[string]string{
		"image/jpeg": ".jpg",
		"image/png":  ".png",
		"image/gif":  ".gif",
	}

	ext, ok := allowedTypes[contentType]
	if !ok {
		return "", errors.New("only JPEG, PNG and GIF images are allowed")
	}

	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		return "", err
	}

	err = os.MkdirAll("./uploads", os.ModePerm)
	if err != nil {
		return "", err
	}

	id, err := uuid.NewV4()
	if err != nil {
		return "", err
	}

	filename := id.String() + ext
	fullPath := filepath.Join("./uploads", filename)

	dst, err := os.Create(fullPath)
	if err != nil {
		return "", err
	}
	defer dst.Close()

	_, err = io.Copy(dst, file)
	if err != nil {
		_ = os.Remove(fullPath)
		return "", err
	}

	log.Printf("Image saved successfully at: %s", fullPath)
	return "/uploads/" + filename, nil
}

type postForm struct {
	title      string
	content    string
	categories []string
}

func newPostPageData(user models.User, allCategories []string, form postForm, errMessage string) PageData {
	return PageData{
		User:                user,
		IsLoggedIn:          true,
		Categories:          allCategories,
		UnreadNotifications: getUnreadNotificationCount(user.ID),
		SelectedCategories:  selectedCategoryMap(form.categories),
		FormAction:          "/post/create",
		SubmitLabel:         "Publish Post",
		PageTitle:           "Create New Post",
		Error:               errMessage,
		Post: models.Post{
			Title:   form.title,
			Content: form.content,
		},
	}
}

func renderCreatePostError(w http.ResponseWriter, data PageData) {
	w.WriteHeader(http.StatusBadRequest)
	_ = renderTemplate(w, "ui/html/create.page.html", data)
}

func parsePostForm(r *http.Request) postForm {
	return postForm{
		title:      r.FormValue("title"),
		content:    r.FormValue("content"),
		categories: r.Form["categories"],
	}
}

func validatePostForm(form postForm) error {
	if strings.TrimSpace(form.title) == "" || strings.TrimSpace(form.content) == "" {
		return errors.New("Title and Content cannot be empty")
	}
	if len(form.categories) == 0 {
		return errors.New("At least one category is required")
	}
	return nil
}

func readPostImage(r *http.Request) (string, error) {
	file, header, err := r.FormFile("image")
	if err == http.ErrMissingFile {
		return "", nil
	}
	if err != nil {
		return "", errors.New("Failed to read uploaded image.")
	}
	defer file.Close()

	if header.Size > maxUploadSize {
		return "", errors.New("Image is too big. Maximum size is 20 MB.")
	}
	return saveUploadedImage(file, header)
}

func insertPost(userID int, form postForm, imagePath string) error {
	tx, err := database.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	result, err := tx.Exec(
		"INSERT INTO posts (user_id, title, content, image_path) VALUES (?, ?, ?, ?)",
		userID, form.title, form.content, imagePath,
	)
	if err != nil {
		return err
	}
	postID, err := result.LastInsertId()
	if err != nil {
		return err
	}

	for _, category := range form.categories {
		result, err := tx.Exec(
			`INSERT INTO post_categories (post_id, category_id)
			 SELECT ?, id FROM categories WHERE name = ?`,
			postID, category,
		)
		if err != nil {
			return err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if affected == 0 {
			return fmt.Errorf("unknown category %q", category)
		}
	}
	return tx.Commit()
}

// CreatePost serves the post form and coordinates validation, upload, and storage.
func CreatePost(w http.ResponseWriter, r *http.Request) {
	user, isLoggedIn := GetUserFromSession(r)
	if !isLoggedIn {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	allCategories := loadCategories()
	if r.Method != http.MethodPost {
		data := newPostPageData(user, allCategories, postForm{}, "")
		if err := renderTemplate(w, "ui/html/create.page.html", data); err != nil {
			ErrorPage(w, http.StatusInternalServerError, "500 - Template Error")
		}
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize+1024*1024)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		data := newPostPageData(user, allCategories, postForm{}, "Image is too big. Maximum size is 20 MB.")
		renderCreatePostError(w, data)
		return
	}

	form := parsePostForm(r)
	if err := validatePostForm(form); err != nil {
		renderCreatePostError(w, newPostPageData(user, allCategories, form, err.Error()))
		return
	}

	imagePath, err := readPostImage(r)
	if err != nil {
		renderCreatePostError(w, newPostPageData(user, allCategories, form, err.Error()))
		return
	}

	if err := insertPost(user.ID, form, imagePath); err != nil {
		log.Printf("Insert Post Error: %v", err)
		removeStoredImage(imagePath)
		ErrorPage(w, http.StatusInternalServerError, "500 - Failed to create post")
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// RatePost applies, toggles, or switches a like/dislike vote on a post.
func RatePost(w http.ResponseWriter, r *http.Request) {
	user, isLoggedIn := GetUserFromSession(r)
	if !isLoggedIn {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	postID := r.FormValue("post_id")
	action := r.FormValue("action")
	if postID == "" {
		ErrorPage(w, http.StatusBadRequest, "400 - Bad Request: Missing Post ID")
		return
	}
	if action != "like" && action != "dislike" {
		ErrorPage(w, http.StatusBadRequest, "400 - Bad Request: Invalid Action")
		return
	}

	var postExists int
	err := database.DB.QueryRow("SELECT 1 FROM posts WHERE id = ?", postID).Scan(&postExists)
	if err == sql.ErrNoRows {
		ErrorPage(w, http.StatusNotFound, "404 - Post Not Found")
		return
	}
	if err != nil {
		ErrorPage(w, http.StatusInternalServerError, "500 - Internal Server Error")
		return
	}

	voteType := 1
	if action == "dislike" {
		voteType = -1
	}

	var existingType int
	err = database.DB.QueryRow("SELECT type FROM votes WHERE user_id = ? AND post_id = ?", user.ID, postID).Scan(&existingType)

	if err == sql.ErrNoRows {
		if _, err := database.DB.Exec("INSERT INTO votes (user_id, post_id, type) VALUES (?, ?, ?)", user.ID, postID, voteType); err != nil {
			ErrorPage(w, http.StatusInternalServerError, "500 - Internal Server Error")
			return
		}
		if voteType == 1 {
			createPostNotification(user.ID, postID, "post_liked")
		} else {
			createPostNotification(user.ID, postID, "post_disliked")
		}
	} else if err != nil {
		ErrorPage(w, http.StatusInternalServerError, "500 - Internal Server Error")
		return
	} else {
		if existingType == voteType {
			if _, err := database.DB.Exec("DELETE FROM votes WHERE user_id = ? AND post_id = ?", user.ID, postID); err != nil {
				ErrorPage(w, http.StatusInternalServerError, "500 - Internal Server Error")
				return
			}
		} else {
			if _, err := database.DB.Exec("UPDATE votes SET type = ? WHERE user_id = ? AND post_id = ?", voteType, user.ID, postID); err != nil {
				ErrorPage(w, http.StatusInternalServerError, "500 - Internal Server Error")
				return
			}
			if voteType == 1 {
				createPostNotification(user.ID, postID, "post_liked")
			} else {
				createPostNotification(user.ID, postID, "post_disliked")
			}
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

// ViewPost renders a post details page together with its comments and counters.
func ViewPost(w http.ResponseWriter, r *http.Request) {
	postID := r.URL.Query().Get("id")
	if postID == "" {
		ErrorPage(w, http.StatusBadRequest, "400 - Bad Request: Missing Post ID")
		return
	}

	user, isLoggedIn := GetUserFromSession(r)

	postUpdatedAtExpr := "p.created_at"
	if tableHasColumn("posts", "updated_at") {
		postUpdatedAtExpr = "p.updated_at"
	}

	query := fmt.Sprintf(`SELECT p.id, p.user_id, p.title, p.content, p.image_path, u.username, p.created_at, %s,
          (SELECT COUNT(*) FROM votes WHERE post_id = p.id AND type = 1) as likes,
          (SELECT COUNT(*) FROM votes WHERE post_id = p.id AND type = -1) as dislikes
          FROM posts p
          JOIN users u ON p.user_id = u.id
          WHERE p.id = ?`, postUpdatedAtExpr)

	var p models.Post
	var imagePath sql.NullString

	err := database.DB.QueryRow(query, postID).Scan(
		&p.ID, &p.UserID, &p.Title, &p.Content, &imagePath, &p.Author, &p.CreatedAt, &p.UpdatedAt, &p.Likes, &p.Dislikes,
	)
	if err == sql.ErrNoRows {
		ErrorPage(w, http.StatusNotFound, "404 - Post Not Found")
		return
	} else if err != nil {
		log.Printf("Database error in ViewPost: %v", err)
		ErrorPage(w, http.StatusInternalServerError, "500 - Internal Server Error")
		return
	}

	if imagePath.Valid {
		p.ImagePath = imagePath.String
	}
	p.CanManage = isLoggedIn && p.UserID == user.ID

	catRows, err := database.DB.Query("SELECT c.name FROM categories c JOIN post_categories pc ON c.id = pc.category_id WHERE pc.post_id = ?", p.ID)
	if err == nil {
		defer catRows.Close()
		for catRows.Next() {
			var cName string
			catRows.Scan(&cName)
			p.Categories = append(p.Categories, cName)
		}
	}

	commentUpdatedAtExpr := "c.created_at"
	if tableHasColumn("comments", "updated_at") {
		commentUpdatedAtExpr = "c.updated_at"
	}

	commentQuery := fmt.Sprintf(`SELECT c.id, c.post_id, c.user_id, c.content, u.username, c.created_at, %s,
                     (SELECT COUNT(*) FROM votes WHERE comment_id = c.id AND type = 1) as likes,
                     (SELECT COUNT(*) FROM votes WHERE comment_id = c.id AND type = -1) as dislikes
                     FROM comments c
                     JOIN users u ON c.user_id = u.id
                     WHERE c.post_id = ?
                     ORDER BY c.created_at ASC`, commentUpdatedAtExpr)

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
		err := rows.Scan(&c.ID, &c.PostID, &c.UserID, &c.Content, &c.Author, &c.CreatedAt, &c.UpdatedAt, &c.Likes, &c.Dislikes)
		if err == nil {
			c.CanManage = isLoggedIn && c.UserID == user.ID
			comments = append(comments, c)
		}
	}

	data := PageData{
		User:                user,
		IsLoggedIn:          isLoggedIn,
		UnreadNotifications: getUnreadNotificationCount(user.ID),
		Post:                p,
		Comments:            comments,
	}

	if err := renderTemplate(w, "ui/html/post.page.html", data); err != nil {
		log.Printf("Template Error in ViewPost: %v", err)
		ErrorPage(w, http.StatusInternalServerError, "500 - Template Error")
	}
}

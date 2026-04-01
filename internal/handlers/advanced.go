package handlers

import (
	"database/sql"
	"fmt"
	"forum/internal/database"
	"forum/internal/models"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

func tableHasColumn(tableName, columnName string) bool {
	rows, err := database.DB.Query(fmt.Sprintf("PRAGMA table_info(%s)", tableName))
	if err != nil {
		return false
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid        int
			name       string
			columnType string
			notNull    int
			defaultVal sql.NullString
			pk         int
		)
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultVal, &pk); err == nil && name == columnName {
			return true
		}
	}

	return false
}

func loadCategories() []string {
	rows, err := database.DB.Query("SELECT name FROM categories ORDER BY name ASC")
	if err != nil {
		return nil
	}
	defer rows.Close()

	var categories []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err == nil {
			categories = append(categories, name)
		}
	}
	return categories
}

func selectedCategoryMap(categories []string) map[string]bool {
	selected := make(map[string]bool, len(categories))
	for _, category := range categories {
		selected[category] = true
	}
	return selected
}

func getUnreadNotificationCount(userID int) int {
	if userID == 0 || database.DB == nil {
		return 0
	}

	var count int
	if err := database.DB.QueryRow("SELECT COUNT(*) FROM notifications WHERE user_id = ? AND is_read = 0", userID).Scan(&count); err != nil {
		return 0
	}
	return count
}

func createPostNotification(actorID int, postID string, notificationType string) {
	if actorID == 0 || postID == "" {
		return
	}

	var ownerID int
	err := database.DB.QueryRow("SELECT user_id FROM posts WHERE id = ?", postID).Scan(&ownerID)
	if err != nil || ownerID == actorID {
		return
	}

	_, err = database.DB.Exec(
		"INSERT INTO notifications (user_id, actor_id, post_id, type) VALUES (?, ?, ?, ?)",
		ownerID, actorID, postID, notificationType,
	)
	if err != nil {
		log.Printf("failed to create notification: %v", err)
	}
}

func fetchNotifications(userID int) ([]models.Notification, error) {
	query := `
		SELECT n.id, n.actor_id, u.username, n.user_id, n.post_id, p.title, n.type, n.is_read, n.created_at
		FROM notifications n
		JOIN users u ON u.id = n.actor_id
		JOIN posts p ON p.id = n.post_id
		WHERE n.user_id = ?
		ORDER BY n.created_at DESC, n.id DESC
		LIMIT 50`

	rows, err := database.DB.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notifications []models.Notification
	for rows.Next() {
		var item models.Notification
		if err := rows.Scan(&item.ID, &item.ActorID, &item.ActorName, &item.UserID, &item.PostID, &item.PostTitle, &item.Type, &item.IsRead, &item.CreatedAt); err == nil {
			notifications = append(notifications, item)
		}
	}
	return notifications, nil
}

func fetchActivityPosts(userID int) ([]models.Post, error) {
	updatedAtExpr := "p.created_at"
	if tableHasColumn("posts", "updated_at") {
		updatedAtExpr = "p.updated_at"
	}

	query := fmt.Sprintf(`
		SELECT p.id, p.user_id, u.username, p.title, p.content, COALESCE(p.image_path, ''), p.created_at, %s,
		       (SELECT COUNT(*) FROM votes WHERE post_id = p.id AND type = 1) AS likes,
		       (SELECT COUNT(*) FROM votes WHERE post_id = p.id AND type = -1) AS dislikes,
		       (SELECT COUNT(*) FROM comments WHERE post_id = p.id) AS comment_count
		FROM posts p
		JOIN users u ON u.id = p.user_id
		WHERE p.user_id = ?
		ORDER BY p.created_at DESC, p.id DESC`, updatedAtExpr)

	rows, err := database.DB.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []models.Post
	for rows.Next() {
		var item models.Post
		if err := rows.Scan(&item.ID, &item.UserID, &item.Author, &item.Title, &item.Content, &item.ImagePath, &item.CreatedAt, &item.UpdatedAt, &item.Likes, &item.Dislikes, &item.CommentCount); err == nil {
			item.CanManage = true
			posts = append(posts, item)
		}
	}
	return posts, nil
}

func fetchActivityVotes(userID int) ([]models.ActivityVote, error) {
	createdAtExpr := "CURRENT_TIMESTAMP"
	orderByClause := "v.id DESC"
	if tableHasColumn("votes", "created_at") {
		createdAtExpr = "v.created_at"
		orderByClause = "v.created_at DESC, v.id DESC"
	}

	query := fmt.Sprintf(`
		SELECT v.id,
		       v.type,
		       CASE WHEN v.post_id IS NOT NULL THEN 'post' ELSE 'comment' END AS target_type,
		       COALESCE(v.post_id, v.comment_id) AS target_id,
		       COALESCE(c.content, p.content, '') AS target_content,
		       COALESCE(p.id, cp.id) AS post_id,
		       COALESCE(p.title, cp.title) AS post_title,
		       %s
		FROM votes v
		LEFT JOIN posts p ON p.id = v.post_id
		LEFT JOIN comments c ON c.id = v.comment_id
		LEFT JOIN posts cp ON cp.id = c.post_id
		WHERE v.user_id = ?
		ORDER BY %s`, createdAtExpr, orderByClause)

	rows, err := database.DB.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var votes []models.ActivityVote
	for rows.Next() {
		var item models.ActivityVote
		var createdAt sql.NullString
		if err := rows.Scan(&item.ID, &item.Type, &item.TargetType, &item.TargetID, &item.TargetContent, &item.PostID, &item.PostTitle, &createdAt); err == nil {
			votes = append(votes, item)
		}
	}
	return votes, nil
}

func fetchActivityComments(userID int) ([]models.Comment, error) {
	updatedAtExpr := "c.created_at"
	if tableHasColumn("comments", "updated_at") {
		updatedAtExpr = "c.updated_at"
	}

	query := fmt.Sprintf(`
		SELECT c.id, c.post_id, c.user_id, u.username, c.content, c.created_at, %s, p.title
		FROM comments c
		JOIN users u ON u.id = c.user_id
		JOIN posts p ON p.id = c.post_id
		WHERE c.user_id = ?
		ORDER BY c.created_at DESC, c.id DESC`, updatedAtExpr)

	rows, err := database.DB.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var comments []models.Comment
	for rows.Next() {
		var item models.Comment
		if err := rows.Scan(&item.ID, &item.PostID, &item.UserID, &item.Author, &item.Content, &item.CreatedAt, &item.UpdatedAt, &item.PostTitle); err == nil {
			item.CanManage = true
			comments = append(comments, item)
		}
	}
	return comments, nil
}

func fetchManagedComments(userID int) ([]models.Comment, error) {
	return fetchActivityComments(userID)
}

func markNotificationsRead(userID int) {
	_, _ = database.DB.Exec("UPDATE notifications SET is_read = 1 WHERE user_id = ? AND is_read = 0", userID)
}

func removeStoredImage(imagePath string) {
	if imagePath == "" || !strings.HasPrefix(imagePath, "/uploads/") {
		return
	}
	_ = os.Remove("." + imagePath)
}

func ActivityPage(w http.ResponseWriter, r *http.Request) {
	user, isLoggedIn := GetUserFromSession(r)
	if !isLoggedIn {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	posts, err := fetchActivityPosts(user.ID)
	if err != nil {
		ErrorPage(w, http.StatusInternalServerError, "500 - Failed to load your posts")
		return
	}
	votes, err := fetchActivityVotes(user.ID)
	if err != nil {
		ErrorPage(w, http.StatusInternalServerError, "500 - Failed to load your votes")
		return
	}
	comments, err := fetchActivityComments(user.ID)
	if err != nil {
		ErrorPage(w, http.StatusInternalServerError, "500 - Failed to load your comments")
		return
	}
	managedComments, err := fetchManagedComments(user.ID)
	if err != nil {
		ErrorPage(w, http.StatusInternalServerError, "500 - Failed to load your content")
		return
	}

	data := PageData{
		User:                user,
		IsLoggedIn:          true,
		UnreadNotifications: getUnreadNotificationCount(user.ID),
		ActivityPosts:       posts,
		ActivityVotes:       votes,
		ActivityComments:    comments,
		ManagedComments:     managedComments,
		PageTitle:           "My Activity",
	}

	if err := renderTemplate(w, "ui/html/activity.page.html", data); err != nil {
		ErrorPage(w, http.StatusInternalServerError, "500 - Template Error")
	}
}

func NotificationsPage(w http.ResponseWriter, r *http.Request) {
	user, isLoggedIn := GetUserFromSession(r)
	if !isLoggedIn {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	notifications, err := fetchNotifications(user.ID)
	if err != nil {
		ErrorPage(w, http.StatusInternalServerError, "500 - Failed to load notifications")
		return
	}
	markNotificationsRead(user.ID)

	data := PageData{
		User:                user,
		IsLoggedIn:          true,
		Notifications:       notifications,
		UnreadNotifications: 0,
		PageTitle:           "Notifications",
	}

	if err := renderTemplate(w, "ui/html/notifications.page.html", data); err != nil {
		ErrorPage(w, http.StatusInternalServerError, "500 - Template Error")
	}
}

func NotificationStream(w http.ResponseWriter, r *http.Request) {
	user, isLoggedIn := GetUserFromSession(r)
	if !isLoggedIn {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	sendCount := func() {
		fmt.Fprintf(w, "data: %d\n\n", getUnreadNotificationCount(user.ID))
		flusher.Flush()
	}

	sendCount()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			sendCount()
		}
	}
}

func EditPost(w http.ResponseWriter, r *http.Request) {
	user, isLoggedIn := GetUserFromSession(r)
	if !isLoggedIn {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	postID := r.URL.Query().Get("id")
	if postID == "" {
		ErrorPage(w, http.StatusBadRequest, "400 - Bad Request: Missing Post ID")
		return
	}

	var post models.Post
	postUpdatedAtExpr := "created_at"
	if tableHasColumn("posts", "updated_at") {
		postUpdatedAtExpr = "updated_at"
	}
	err := database.DB.QueryRow(
		fmt.Sprintf("SELECT id, user_id, title, content, COALESCE(image_path, ''), created_at, %s FROM posts WHERE id = ?", postUpdatedAtExpr),
		postID,
	).Scan(&post.ID, &post.UserID, &post.Title, &post.Content, &post.ImagePath, &post.CreatedAt, &post.UpdatedAt)
	if err == sql.ErrNoRows {
		ErrorPage(w, http.StatusNotFound, "404 - Post Not Found")
		return
	}
	if err != nil {
		ErrorPage(w, http.StatusInternalServerError, "500 - Internal Server Error")
		return
	}
	if post.UserID != user.ID {
		ErrorPage(w, http.StatusForbidden, "403 - You can only edit your own posts")
		return
	}

	rows, err := database.DB.Query("SELECT c.name FROM categories c JOIN post_categories pc ON pc.category_id = c.id WHERE pc.post_id = ?", postID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var name string
			if rows.Scan(&name) == nil {
				post.Categories = append(post.Categories, name)
			}
		}
	}

	allCats := loadCategories()
	renderEditPost := func(errMsg string) {
		data := PageData{
			User:                user,
			IsLoggedIn:          true,
			Categories:          allCats,
			Post:                post,
			Error:               errMsg,
			FormAction:          "/post/edit?id=" + postID,
			SubmitLabel:         "Save Changes",
			PageTitle:           "Edit Post",
			SelectedCategories:  selectedCategoryMap(post.Categories),
			UnreadNotifications: getUnreadNotificationCount(user.ID),
		}
		if err := renderTemplate(w, "ui/html/create.page.html", data); err != nil {
			ErrorPage(w, http.StatusInternalServerError, "500 - Template Error")
		}
	}

	if r.Method == http.MethodGet {
		renderEditPost("")
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize+1024*1024)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		renderEditPost("Image is too big. Maximum size is 20 MB.")
		return
	}

	post.Title = r.FormValue("title")
	post.Content = r.FormValue("content")
	selectedCategories := r.Form["categories"]
	post.Categories = selectedCategories
	newImagePath := post.ImagePath

	file, header, err := r.FormFile("image")
	if err == nil {
		defer file.Close()
		if header.Size > maxUploadSize {
			renderEditPost("Image is too big. Maximum size is 20 MB.")
			return
		}
		uploadedPath, uploadErr := saveUploadedImage(file, header)
		if uploadErr != nil {
			renderEditPost(uploadErr.Error())
			return
		}
		if post.ImagePath != "" && post.ImagePath != uploadedPath {
			removeStoredImage(post.ImagePath)
		}
		newImagePath = uploadedPath
	} else if err != http.ErrMissingFile {
		renderEditPost("Failed to read uploaded image.")
		return
	}

	if strings.TrimSpace(post.Title) == "" || strings.TrimSpace(post.Content) == "" {
		renderEditPost("Title and Content cannot be empty")
		return
	}
	if len(selectedCategories) == 0 {
		renderEditPost("At least one category is required")
		return
	}

	tx, err := database.DB.Begin()
	if err != nil {
		ErrorPage(w, http.StatusInternalServerError, "500 - Internal Server Error")
		return
	}
	defer tx.Rollback()

	updatePostQuery := "UPDATE posts SET title = ?, content = ?, image_path = ? WHERE id = ?"
	if tableHasColumn("posts", "updated_at") {
		updatePostQuery = "UPDATE posts SET title = ?, content = ?, image_path = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?"
	}
	if _, err := tx.Exec(updatePostQuery, post.Title, post.Content, newImagePath, postID); err != nil {
		ErrorPage(w, http.StatusInternalServerError, "500 - Failed to update post")
		return
	}
	if _, err := tx.Exec("DELETE FROM post_categories WHERE post_id = ?", postID); err != nil {
		ErrorPage(w, http.StatusInternalServerError, "500 - Failed to update post")
		return
	}
	for _, catName := range selectedCategories {
		if _, err := tx.Exec(`INSERT INTO post_categories (post_id, category_id)
			SELECT ?, id FROM categories WHERE name = ?`, postID, catName); err != nil {
			ErrorPage(w, http.StatusInternalServerError, "500 - Failed to update post")
			return
		}
	}
	if err := tx.Commit(); err != nil {
		ErrorPage(w, http.StatusInternalServerError, "500 - Failed to update post")
		return
	}

	http.Redirect(w, r, "/post?id="+postID, http.StatusSeeOther)
}

func DeletePost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	user, isLoggedIn := GetUserFromSession(r)
	if !isLoggedIn {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	postID := r.FormValue("post_id")
	if postID == "" {
		ErrorPage(w, http.StatusBadRequest, "400 - Bad Request: Missing Post ID")
		return
	}

	var ownerID int
	var imagePath string
	err := database.DB.QueryRow("SELECT user_id, COALESCE(image_path, '') FROM posts WHERE id = ?", postID).Scan(&ownerID, &imagePath)
	if err == sql.ErrNoRows {
		ErrorPage(w, http.StatusNotFound, "404 - Post Not Found")
		return
	}
	if err != nil {
		ErrorPage(w, http.StatusInternalServerError, "500 - Internal Server Error")
		return
	}
	if ownerID != user.ID {
		ErrorPage(w, http.StatusForbidden, "403 - You can only remove your own posts")
		return
	}

	if _, err := database.DB.Exec("DELETE FROM posts WHERE id = ?", postID); err != nil {
		ErrorPage(w, http.StatusInternalServerError, "500 - Failed to delete post")
		return
	}
	removeStoredImage(imagePath)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func EditComment(w http.ResponseWriter, r *http.Request) {
	user, isLoggedIn := GetUserFromSession(r)
	if !isLoggedIn {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	commentID := r.URL.Query().Get("id")
	if commentID == "" {
		ErrorPage(w, http.StatusBadRequest, "400 - Bad Request: Missing Comment ID")
		return
	}

	var comment models.Comment
	err := database.DB.QueryRow(
		`SELECT c.id, c.post_id, c.user_id, c.content, c.created_at, c.updated_at, p.title
		 FROM comments c
		 JOIN posts p ON p.id = c.post_id
		 WHERE c.id = ?`,
		commentID,
	).Scan(&comment.ID, &comment.PostID, &comment.UserID, &comment.Content, &comment.CreatedAt, &comment.UpdatedAt, &comment.PostTitle)
	if err == sql.ErrNoRows {
		ErrorPage(w, http.StatusNotFound, "404 - Comment Not Found")
		return
	}
	if err != nil {
		ErrorPage(w, http.StatusInternalServerError, "500 - Internal Server Error")
		return
	}
	if comment.UserID != user.ID {
		ErrorPage(w, http.StatusForbidden, "403 - You can only edit your own comments")
		return
	}

	render := func(errMsg string) {
		data := PageData{
			User:                user,
			IsLoggedIn:          true,
			Comments:            []models.Comment{comment},
			Error:               errMsg,
			PageTitle:           "Edit Comment",
			UnreadNotifications: getUnreadNotificationCount(user.ID),
		}
		if err := renderTemplate(w, "ui/html/edit-comment.page.html", data); err != nil {
			ErrorPage(w, http.StatusInternalServerError, "500 - Template Error")
		}
	}

	if r.Method == http.MethodGet {
		render("")
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	comment.Content = r.FormValue("content")
	if strings.TrimSpace(comment.Content) == "" {
		render("Comment cannot be empty")
		return
	}

	if _, err := database.DB.Exec("UPDATE comments SET content = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", comment.Content, commentID); err != nil {
		ErrorPage(w, http.StatusInternalServerError, "500 - Failed to update comment")
		return
	}

	http.Redirect(w, r, "/post?id="+strconv.Itoa(comment.PostID), http.StatusSeeOther)
}

func DeleteComment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	user, isLoggedIn := GetUserFromSession(r)
	if !isLoggedIn {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	commentID := r.FormValue("comment_id")
	if commentID == "" {
		ErrorPage(w, http.StatusBadRequest, "400 - Bad Request: Missing Comment ID")
		return
	}

	var ownerID, postID int
	err := database.DB.QueryRow("SELECT user_id, post_id FROM comments WHERE id = ?", commentID).Scan(&ownerID, &postID)
	if err == sql.ErrNoRows {
		ErrorPage(w, http.StatusNotFound, "404 - Comment Not Found")
		return
	}
	if err != nil {
		ErrorPage(w, http.StatusInternalServerError, "500 - Internal Server Error")
		return
	}
	if ownerID != user.ID {
		ErrorPage(w, http.StatusForbidden, "403 - You can only remove your own comments")
		return
	}

	if _, err := database.DB.Exec("DELETE FROM comments WHERE id = ?", commentID); err != nil {
		ErrorPage(w, http.StatusInternalServerError, "500 - Failed to delete comment")
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

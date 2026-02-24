package handlers

import (
	"database/sql"
	"forum/internal/database"
	"net/http"
	"strings"
)

func CreateComment(w http.ResponseWriter, r *http.Request) {
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
	content := r.FormValue("content")

	// Audit Check: Empty comment
	if strings.TrimSpace(content) == "" {
		http.Error(w, "Comment cannot be empty", http.StatusBadRequest)
		return
	}

	_, err := database.DB.Exec("INSERT INTO comments (post_id, user_id, content) VALUES (?, ?, ?)", postID, user.ID, content)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Επιστροφή στο post
	http.Redirect(w, r, "/post?id="+postID, http.StatusSeeOther)
}

// RateComment handles Likes/Dislikes for comments
func RateComment(w http.ResponseWriter, r *http.Request) {
	user, isLoggedIn := GetUserFromSession(r)
	if !isLoggedIn {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	commentID := r.FormValue("comment_id")
	action := r.FormValue("action") // "like" or "dislike"
	voteType := 1
	if action == "dislike" {
		voteType = -1
	}

	// 1. Check existing vote specifically for this COMMENT (comment_id)
	var existingType int
	// Προσοχή: Ελέγχουμε το comment_id, όχι το post_id εδώ
	err := database.DB.QueryRow("SELECT type FROM votes WHERE user_id = ? AND comment_id = ?", user.ID, commentID).Scan(&existingType)

	if err == sql.ErrNoRows { // Ή sql.ErrNoRows αν έχεις κάνει import το database/sql
		// New Vote
		database.DB.Exec("INSERT INTO votes (user_id, comment_id, type) VALUES (?, ?, ?)", user.ID, commentID, voteType)
	} else {
		// Existing Vote logic (Toggle or Change)
		if existingType == voteType {
			// Clicked same button -> Remove vote (Toggle off)
			database.DB.Exec("DELETE FROM votes WHERE user_id = ? AND comment_id = ?", user.ID, commentID)
		} else {
			// Clicked different button -> Update vote
			database.DB.Exec("UPDATE votes SET type = ? WHERE user_id = ? AND comment_id = ?", voteType, user.ID, commentID)
		}
	}

	// Redirect back to where they came from (the post page)
	http.Redirect(w, r, r.Header.Get("Referer"), http.StatusSeeOther)
}

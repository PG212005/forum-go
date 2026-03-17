package handlers

import (
	"database/sql"
	"forum/internal/database"
	"log"
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
	if strings.TrimSpace(postID) == "" {
		ErrorPage(w, http.StatusBadRequest, "400 - Bad Request: Missing Post ID")
		return
	}

	// Reject comments that are empty after trimming whitespace.
	if strings.TrimSpace(content) == "" {
		ErrorPage(w, http.StatusBadRequest, "400 - Bad Request: Empty Comment")
		return
	}

	_, err := database.DB.Exec("INSERT INTO comments (post_id, user_id, content) VALUES (?, ?, ?)", postID, user.ID, content)
	if err != nil {
		// Log the underlying database error for debugging while returning a generic 500 to the client.
		log.Printf("Internal Server Error: %v", err)

		ErrorPage(w, http.StatusInternalServerError, "500 - Internal Server Error")
		return
	}

	// Return the user to the post after the comment is created.
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
	if commentID == "" {
		ErrorPage(w, http.StatusBadRequest, "400 - Bad Request: Missing Comment ID")
		return
	}
	if action != "like" && action != "dislike" {
		ErrorPage(w, http.StatusBadRequest, "400 - Bad Request: Invalid Action")
		return
	}

	var commentExists int
	err := database.DB.QueryRow("SELECT 1 FROM comments WHERE id = ?", commentID).Scan(&commentExists)
	if err == sql.ErrNoRows {
		ErrorPage(w, http.StatusNotFound, "404 - Comment Not Found")
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

	// Look up an existing vote for this specific comment.
	var existingType int
	err = database.DB.QueryRow("SELECT type FROM votes WHERE user_id = ? AND comment_id = ?", user.ID, commentID).Scan(&existingType)

	if err == sql.ErrNoRows {
		// Insert a new vote when the user has not voted on this comment yet.
		if _, err := database.DB.Exec("INSERT INTO votes (user_id, comment_id, type) VALUES (?, ?, ?)", user.ID, commentID, voteType); err != nil {
			ErrorPage(w, http.StatusInternalServerError, "500 - Internal Server Error")
			return
		}
	} else if err != nil {
		ErrorPage(w, http.StatusInternalServerError, "500 - Internal Server Error")
		return
	} else {
		// Toggle the vote off when the same action is repeated, otherwise switch the vote type.
		if existingType == voteType {
			if _, err := database.DB.Exec("DELETE FROM votes WHERE user_id = ? AND comment_id = ?", user.ID, commentID); err != nil {
				ErrorPage(w, http.StatusInternalServerError, "500 - Internal Server Error")
				return
			}
		} else {
			if _, err := database.DB.Exec("UPDATE votes SET type = ? WHERE user_id = ? AND comment_id = ?", voteType, user.ID, commentID); err != nil {
				ErrorPage(w, http.StatusInternalServerError, "500 - Internal Server Error")
				return
			}
		}
	}

	// Send the user back to the page they came from after voting.
	http.Redirect(w, r, r.Header.Get("Referer"), http.StatusSeeOther)
}

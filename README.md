README.md
Markdown

# Forum Project - Golang & SQLite

A robust, web-based forum application designed for user interaction, content creation, and secure authentication. This project demonstrates the implementation of a full-stack Go application using a monolithic architecture.

## 🚀 Key Features
- **Secure Authentication**: Registration and Login using `bcrypt` for password hashing and `UUID` for session tokens.
- **Interactive Content**: Create posts with multiple categories and nested comments.
- **Voting System**: Integrated Like/Dislike functionality for both posts and comments with "toggle" logic.
- **Dynamic Filtering**: View posts by category, your own created posts, or posts you have liked.
- **Middleware Security**: Route protection and session validation for all sensitive actions.
- **Data Integrity**: SQLite database with Foreign Key constraints and cascading deletes.
- **Containerization**: Fully Dockerized environment for consistent deployment.

## 🛠 Tech Stack
- **Backend**: Go (Golang)
- **Database**: SQLite3
- **Security**: Bcrypt (Hashing), UUID v4 (Sessions)
- **Frontend**: HTML5, CSS3 (Go Templates)
- **DevOps**: Docker & Docker Compose

## ⚙️ Execution Guide

### 1. Using Docker (Recommended)
Build and run the entire stack:
```bash
docker build -t forum-app .
docker run --rm -p 8080:8080   -v "$(pwd)/forum.db":/app/forum.db   -v "$(pwd)/uploads":/app/uploads   forum-app
2. Running Locally
Bash

go run cmd/main.go

The server will start at http://localhost:8080.
📁 Project Structure

    cmd/: Application entry point.

    internal/database/: Schema initialization and DB connection.

    internal/handlers/: Business logic for every route.

    internal/middleware/: Authentication and security filters.

    ui/: Frontend templates and static assets.


---

## 🔍 Audit Preparation: Commands & Steps

Use these specific commands during the audit to prove your project meets the requirements.

### 1. Database & SQL Audit
To prove the existence of `CREATE`, `INSERT`, and `SELECT` queries:

* **Verify Schema (CREATE queries):**
  ```bash
  sqlite3 forum.db ".schema"

    Verify Registered User (INSERT/SELECT):
    Bash

sqlite3 forum.db "SELECT id, username, email FROM users;"

Verify Encrypted Passwords:
Bash

    sqlite3 forum.db "SELECT password FROM users LIMIT 1;"
 

2. Docker Audit

To prove your containerization is working:

    Build the image:
    Bash

docker build -t forum-image .

List images:
Bash

docker images

Run and verify container:
Bash

    docker run -d -p 8080:8080 --name forum-instance forum-image
    docker ps -a

Check Users:**
    ```bash
    sqlite3 forum.db "SELECT id, username, email FROM users;"
    ```
* **Check Hashed Passwords (Security Audit):**
    ```bash
    sqlite3 forum.db "SELECT password FROM users;"
    sqlite3 forum.db "SELECT * FROM posts;"
    sqlite3 forum.db "SELECT * FROM comments;"

🐳 Docker Section

1. Does the project have Dockerfiles?

    Action: Show the file named Dockerfile (and docker-compose.yaml if you have one) in your root directory.

2. Build the Image

    Command: ```bash
    docker build -t forum-image .

    Verify Image List:
    Bash

    docker images

    Check if forum-image appears with a Size and Image ID.

3. Run the Container

    Command: ```bash
    docker run -d -p 8080:8080 --name forum-container forum-image

    Verify Running Containers:
    Bash

    docker ps -a

    Ensure the STATUS says "Up" and the PORTS show "0.0.0.0:8080->8080/tcp".

4. Does the project have no unused objects?

    Action: Auditors look for "dangling" images or stopped containers. To clean up before they look, run:
    Bash

docker system prune -f

Then show them docker images and docker ps -a to prove only your active project objects exist.
. Σταμάτησε και διάγραψε το Container

Αυτή η εντολή σταματάει το container forum-test και το αφαιρεί από τη λίστα του docker ps -a.
Bash

docker rm -f forum-test

2. Διάγραψε το Image (Προαιρετικό αλλά προτείνεται)

Αν θέλεις να δείξεις στον auditor ότι το project γίνεται build γρήγορα και σωστά, διέγραψε την εικόνα:
Bash

docker rmi forum-app

3. Καθάρισε τα πάντα (The Deep Clean)

Για να μην υπάρχει ούτε ίχνος από παλιά πειράματα ή "ορφανά" images (dangling images) που πιάνουν χώρο:
Bash

docker system prune -f
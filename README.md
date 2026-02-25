# Go Forum Project

A fully functional forum application written in Go, using SQLite and Docker.

## Features
- User Authentication (Register/Login) with Session Management
- Create Posts & Comments
- Like & Dislike System
- Filter by Categories, Created Posts, and Liked Posts
- Dockerized Environment with Data Persistence

## How to Run (Docker)

1. **Build the Image:**
   ```bash
   docker build -t forum-app .

    Run the Container:
    (This command ensures data persistence via volume mapping)
    Bash

    docker container run -p 8080:8080 -v "$(pwd)/forum.db":/app/forum.db forum-app

    Access the Forum:
    Open http://localhost:8080 in your browser.

Technologies

    Language: Go (Golang)

    Database: SQLite3

    Security: Bcrypt (Passwords), UUID (Sessions)

    Frontend: HTML/CSS
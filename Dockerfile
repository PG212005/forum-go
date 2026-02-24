# Stage 1: Build
FROM golang:1.20-alpine AS builder

# Απαραίτητα για SQLite (χρειάζεται gcc)
RUN apk add --no-cache gcc musl-dev

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build το binary (με CGO enabled για SQLite)
RUN CGO_ENABLED=1 GOOS=linux go build -o forum-server ./cmd/main.go

# Stage 2: Run
FROM alpine:latest

WORKDIR /app

# Αντιγραφή του binary από το Stage 1
COPY --from=builder /app/forum-server .
# Αντιγραφή των HTML templates & CSS
COPY --from=builder /app/ui ./ui

# Έκθεση της θύρας
EXPOSE 8080

# Εκκίνηση
CMD ["./forum-server"]
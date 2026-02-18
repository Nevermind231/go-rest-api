package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type User struct {
	ID        int       `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type Profile struct {
	ID     int    `json:"id"`
	UserID int    `json:"user_id"`
	Bio    string `json:"bio"`
	Age    int    `json:"age"`
}

func main() {
	ctx := context.Background()

	if err := run(ctx); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) error {

	dsn := "postgres://myuser:mypassword@localhost:5432/mydb?sslmode=disable"

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return err
	}

	if err := db.PingContext(ctx); err != nil {
		return err
	}

	mux := http.NewServeMux()

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	mux.HandleFunc("/users", createUserHandler(db))
	mux.HandleFunc("/users/", userByIDHandler(db))
	mux.HandleFunc("/profiles", createProfileHandler(db))
	mux.HandleFunc("/profiles/", profileByIDHandler(db))

	return server.ListenAndServe()

}

func createUserHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		type request struct {
			Email string `json:"email"`
			Name  string `json:"name"`
		}

		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		query := `
    INSERT INTO users (email, name)
    VALUES ($1, $2)
    RETURNING id
`
		var id int
		err := db.QueryRowContext(r.Context(), query, req.Email, req.Name).Scan(&id)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]int{"id": id})
	}
}

func userByIDHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		idStr := strings.TrimPrefix(r.URL.Path, "/users/")
		if idStr == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		id, err := strconv.Atoi(idStr)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		switch r.Method {

		case http.MethodGet:
			var user User

			query := `
			SELECT id,email,name,created_at
			FROM users
			WHERE id=$1
			`

			err = db.QueryRowContext(r.Context(), query, id).
				Scan(&user.ID, &user.Email, &user.Name, &user.CreatedAt)

			if err == sql.ErrNoRows {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(user)

		case http.MethodPut:
			type request struct {
				Name string `json:"name"`
			}

			var req request
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			if req.Name == "" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			query := `
			UPDATE users
			SET name = $1
			WHERE id = $2
			`

			result, err := db.ExecContext(r.Context(), query, req.Name, id)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			rows, err := result.RowsAffected()
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			if rows == 0 {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			w.WriteHeader(http.StatusNoContent)

		case http.MethodDelete:

			query := `
	DELETE FROM users
	WHERE id = $1
	`

			result, err := db.ExecContext(r.Context(), query, id)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			rows, err := result.RowsAffected()
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			if rows == 0 {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			w.WriteHeader(http.StatusNoContent)

		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

	}
}

func createProfileHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		type request struct {
			UserID int    `json:"user_id"`
			Bio    string `json:"bio"`
			Age    int    `json:"age"`
		}

		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if req.UserID == 0 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		query := `
INSERT INTO profiles (user_id, bio, age)
VALUES ($1, $2, $3)
RETURNING id
`

		var id int
		err := db.QueryRowContext(r.Context(), query, req.UserID, req.Bio, req.Age).Scan(&id)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]int{"id": id})
	}
}

func profileByIDHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		idStr := strings.TrimPrefix(r.URL.Path, "/profiles/")
		if idStr == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		id, err := strconv.Atoi(idStr)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		var profile Profile

		query := `
SELECT id, user_id, bio, age
FROM profiles
WHERE ID =$1
`
		err = db.QueryRowContext(r.Context(), query, id).
			Scan(&profile.ID, &profile.UserID, &profile.Bio, &profile.Age)
		if err == sql.ErrNoRows {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(profile)
	}
}

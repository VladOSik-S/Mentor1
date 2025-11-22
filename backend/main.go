package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"
)

var db *sql.DB

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}
	var err error
	db, err = initDB()
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Close()

	if err := goose.SetDialect("postgres"); err != nil {
		log.Fatal("Error:", err)
	}
	if err := goose.Up(db, "migrations"); err != nil {
		log.Fatal("Failed to run migrations:", err)
	}
	log.Println("✓ Migrations applied")

	mux := http.NewServeMux()

	// Auth routes
	mux.HandleFunc("/api/auth/login", handleLogin)

	// Task solution routes
	mux.HandleFunc("/api/tasks/{id}/solutions", handleTaskSolutions)

	// Admin routes
	mux.HandleFunc("/api/admin/users", adminGetUsers)

	http.HandleFunc("/api/hh/settings", handleHHSettings)
	http.HandleFunc("/api/hh/cover-letters", handleCoverLetters)
	http.HandleFunc("/api/hh/cover-letters/", handleCoverLetter)
	http.HandleFunc("/api/hh/applications", handleApplications)
	http.HandleFunc("/api/hh/stats", getApplicationStats)

	mux.HandleFunc("/api/calendar/events", getCalendarEvents)
	mux.HandleFunc("/api/calendar/events/create", createPersonalEvent)
	mux.HandleFunc("/api/calendar/events/", func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) >= 4 {
			if len(parts) == 5 && parts[4] == "hide" {
				hideGlobalEvent(w, r)
			} else if r.Method == http.MethodPut {
				updatePersonalEvent(w, r)
			} else if r.Method == http.MethodDelete {
				deletePersonalEvent(w, r)
			} else {
				respondError(w, http.StatusNotFound, "Not found")
			}
		}
	})

	// Единый обработчик для всех подмаршрутов /api/admin/users/
	mux.HandleFunc("/api/admin/users/", func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")

		// Check if this is a note-tags route
		if len(parts) >= 5 && parts[4] == "note-tags" {
			if len(parts) == 5 {
				// GET /api/admin/users/:id/note-tags
				// POST /api/admin/users/:id/note-tags
				switch r.Method {
				case http.MethodGet:
					adminGetUserNoteTags(w, r)
				case http.MethodPost:
					adminAssignNoteTag(w, r)
				default:
					respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
				}
			} else if len(parts) == 6 {
				// DELETE /api/admin/users/:id/note-tags/:tag
				if r.Method == http.MethodDelete {
					adminUnassignNoteTag(w, r)
				} else {
					respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
				}
			}
			return
		}

		// Check if this is a question-tags route
		if len(parts) >= 5 && parts[4] == "question-tags" {
			if len(parts) == 5 {
				// GET /api/admin/users/:id/question-tags
				// POST /api/admin/users/:id/question-tags
				switch r.Method {
				case http.MethodGet:
					adminGetUserQuestionTags(w, r)
				case http.MethodPost:
					adminAssignQuestionTag(w, r)
				default:
					respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
				}
			} else if len(parts) == 6 {
				// DELETE /api/admin/users/:id/question-tags/:tag
				if r.Method == http.MethodDelete {
					adminUnassignQuestionTag(w, r)
				} else {
					respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
				}
			}
			return
		}

		// Otherwise handle as user detail route
		adminGetUserDetails(w, r)
	})

	mux.HandleFunc("/api/admin/sprints", adminHandleSprints) // GET, POST
	mux.HandleFunc("/api/admin/sprints/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			adminUpdateSprint(w, r) // PUT /api/admin/sprints/:id
		} else {
			adminHandleSprint(w, r) // DELETE /api/admin/sprints/:id
		}
	})
	mux.HandleFunc("/api/admin/tasks", adminHandleTasks) // GET, POST
	mux.HandleFunc("/api/admin/tasks/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			adminUpdateTask(w, r) // PUT /api/admin/tasks/:id
		} else {
			adminHandleTask(w, r) // DELETE /api/admin/tasks/:id
		}
	})
	mux.HandleFunc("/api/admin/assign-sprint", adminAssignSprint)
	mux.HandleFunc("/api/admin/unassign-sprint", adminUnassignSprint)
	mux.HandleFunc("/api/admin/solutions", adminGetSolutions)
	mux.HandleFunc("/api/admin/solutions/", adminReviewSolution)
	mux.HandleFunc("/api/admin/notes", adminHandleNotes) // GET, POST
	mux.HandleFunc("/api/admin/notes/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			adminUpdateNote(w, r) // PUT /api/admin/notes/:id
		} else {
			adminDeleteNote(w, r) // DELETE /api/admin/notes/:id
		}
	})

	// Question routes
	mux.HandleFunc("/api/questions", getQuestions)
	mux.HandleFunc("/api/questions/tags", getQuestionTags)
	mux.HandleFunc("/api/questions/", answerQuestion)

	// Admin question routes
	mux.HandleFunc("/api/admin/questions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			adminGetQuestions(w, r)
		} else if r.Method == http.MethodPost {
			adminCreateQuestion(w, r)
		}
	})
	mux.HandleFunc("/api/admin/questions/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			adminUpdateQuestion(w, r)
		} else if r.Method == http.MethodDelete {
			adminDeleteQuestion(w, r)
		}
	})

	// User routes
	mux.HandleFunc("/api/users/me", getUserMe)
	mux.HandleFunc("/api/users/me/stats", getUserStats)
	mux.HandleFunc("/api/users/me/settings", handleSettings)

	// Sprint routes
	mux.HandleFunc("/api/sprints", getSprints)
	mux.HandleFunc("/api/sprints/", handleSprint)

	// Task routes
	mux.HandleFunc("/api/tasks/", handleTask)

	// Note routes
	mux.HandleFunc("/api/notes", handleNotes)
	mux.HandleFunc("/api/notes/tags", getNoteTags)
	mux.HandleFunc("/api/notes/", handleNote)

	// Achievement routes
	mux.HandleFunc("/api/achievements", getAchievements)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	handler := corsMiddleware(mux)

	log.Printf("🚀 Server starting on http://localhost:%s", port)
	if err := http.ListenAndServe(":"+port, handler); err != nil {
		log.Fatal(err)
	}
}

func initDB() (*sql.DB, error) {
	if databaseURL := os.Getenv("DATABASE_URL"); databaseURL != "" {
		db, err := sql.Open("postgres", databaseURL)
		if err != nil {
			return nil, err
		}

		db.SetMaxOpenConns(25)
		db.SetMaxIdleConns(5)
		db.SetConnMaxLifetime(5 * time.Minute)

		for i := 0; i < 30; i++ {
			if err = db.Ping(); err == nil {
				log.Println("✓ Database connected (DATABASE_URL)")
				return db, nil
			}
			log.Printf("⏳ Waiting for database... (%d/30)\n", i+1)
			time.Sleep(time.Second)
		}
		return nil, fmt.Errorf("failed to connect to database after 30 attempts: %w", err)
	}

	host := getEnv("DB_HOST", "localhost")
	port := getEnv("DB_PORT", "5432")
	user := getEnv("DB_USER", "postgres")
	password := getEnv("DB_PASSWORD", "postgres")
	dbname := getEnv("DB_NAME", "learning_platform")

	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)

	log.Printf("📊 Connecting to database at %s:%s/%s", host, port, dbname)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	for i := 0; i < 30; i++ {
		if err = db.Ping(); err == nil {
			log.Println("✓ Database connected")
			return db, nil
		}
		log.Printf("⏳ Waiting for database... (%d/30)\n", i+1)
		time.Sleep(time.Second)
	}

	return nil, fmt.Errorf("failed to connect to database after 30 attempts: %w", err)
}

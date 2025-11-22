package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func handleNotes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	userID := getUserID(r)

	// Проверяем роль пользователя
	var role string
	db.QueryRow(`SELECT role FROM users WHERE id = $1`, userID).Scan(&role)

	// Получаем назначенные пользователю теги
	var assignedTags []string
	rows, err := db.Query(`SELECT tag FROM user_note_tags WHERE user_id = $1`, userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer rows.Close()

	for rows.Next() {
		var tag string
		rows.Scan(&tag)
		assignedTags = append(assignedTags, tag)
	}

	search := r.URL.Query().Get("search")
	view := r.URL.Query().Get("view")
	tags := r.URL.Query()["tags"]

	// Если у пользователя нет назначенных тегов, возвращаем только публичные заметки
	if len(assignedTags) == 0 && role != "admin" {
		query := `
			SELECT n.id, n.title, n.tags, n.created_at, n.updated_at,
				   COALESCE(un.favorite, false), COALESCE(un.read, false)
			FROM notes n
			LEFT JOIN user_notes un ON n.id = un.note_id AND un.user_id = $1
			WHERE n.is_public = true
		`
		args := []interface{}{userID}
		argCount := 1

		// Добавляем фильтры для публичных заметок
		if search != "" {
			argCount++
			query += fmt.Sprintf(` AND (n.title ILIKE $%d OR n.content ILIKE $%d)`, argCount, argCount)
			args = append(args, "%"+search+"%")
		}

		if len(tags) > 0 {
			argCount++
			query += fmt.Sprintf(` AND n.tags && $%d`, argCount)
			args = append(args, fmt.Sprintf("{%s}", strings.Join(tags, ",")))
		}

		// КЛЮЧЕВОЕ ИСПРАВЛЕНИЕ: Применяем фильтры view для публичных заметок
		if view == "unread" {
			query += ` AND COALESCE(un.read, false) = false`
		} else if view == "favorites" {
			query += ` AND un.favorite = true`
		}

		query += ` ORDER BY n.created_at DESC`

		rows, err := db.Query(query, args...)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Database error")
			return
		}
		defer rows.Close()

		notes := []Note{}
		for rows.Next() {
			var n Note
			var tagsArray []string
			rows.Scan(&n.ID, &n.Title, (*StringArray)(&tagsArray), &n.CreatedAt,
				&n.UpdatedAt, &n.Favorite, &n.Read)
			n.Tags = tagsArray
			notes = append(notes, n)
		}

		respondJSON(w, http.StatusOK, map[string]interface{}{
			"notes":    notes,
			"is_guest": role == "guest",
			"limit":    len(assignedTags) == 0,
		})
		return
	}

	query := `
        SELECT n.id, n.title, n.tags, n.created_at, n.updated_at,
               COALESCE(un.favorite, false), COALESCE(un.read, false)
        FROM notes n
        LEFT JOIN user_notes un ON n.id = un.note_id AND un.user_id = $1
        WHERE 1=1
    `
	args := []interface{}{userID}
	argCount := 1

	if role != "admin" && len(assignedTags) > 0 {
		argCount++
		query += fmt.Sprintf(` AND (n.tags && $%d OR n.is_public = true)`, argCount)
		args = append(args, fmt.Sprintf("{%s}", strings.Join(assignedTags, ",")))
	}

	if search != "" {
		argCount++
		query += fmt.Sprintf(` AND (n.title ILIKE $%d OR n.content ILIKE $%d)`, argCount, argCount)
		args = append(args, "%"+search+"%")
	}

	if len(tags) > 0 {
		argCount++
		query += fmt.Sprintf(` AND n.tags && $%d`, argCount)
		args = append(args, fmt.Sprintf("{%s}", strings.Join(tags, ",")))
	}

	if view == "unread" {
		query += ` AND COALESCE(un.read, false) = false`
	} else if view == "favorites" {
		query += ` AND un.favorite = true`
	}

	query += ` ORDER BY n.created_at DESC`

	rows, err = db.Query(query, args...)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer rows.Close()

	notes := []Note{}
	for rows.Next() {
		var n Note
		var tagsArray []string
		rows.Scan(&n.ID, &n.Title, (*StringArray)(&tagsArray), &n.CreatedAt,
			&n.UpdatedAt, &n.Favorite, &n.Read)
		n.Tags = tagsArray
		notes = append(notes, n)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"notes":    notes,
		"is_guest": role == "guest",
		"limit":    len(assignedTags) == 0,
	})
}

func getNoteTags(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	userID := getUserID(r)

	// Получаем роль пользователя
	var role string
	db.QueryRow(`SELECT role FROM users WHERE id = $1`, userID).Scan(&role)

	// Если админ - показываем все теги
	if role == "admin" {
		rows, err := db.Query(`SELECT DISTINCT unnest(tags) as tag FROM notes ORDER BY tag`)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Database error")
			return
		}
		defer rows.Close()

		tags := []string{}
		for rows.Next() {
			var tag string
			rows.Scan(&tag)
			tags = append(tags, tag)
		}

		respondJSON(w, http.StatusOK, map[string]interface{}{"tags": tags})
		return
	}

	// Для не-админов - только назначенные теги
	rows, err := db.Query(`SELECT tag FROM user_note_tags WHERE user_id = $1 ORDER BY tag`, userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer rows.Close()

	tags := []string{}
	for rows.Next() {
		var tag string
		rows.Scan(&tag)
		tags = append(tags, tag)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"tags": tags})
}

func handleNote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 3 {
		respondError(w, http.StatusBadRequest, "Invalid note identifier")
		return
	}

	userID := getUserID(r)

	// Проверяем роль пользователя один раз
	var role string
	db.QueryRow(`SELECT role FROM users WHERE id = $1`, userID).Scan(&role)

	// Определяем что это - ID или название
	var noteID int
	var noteTitle string

	// Пробуем парсить как число
	id, err := strconv.Atoi(parts[2])
	if err == nil {
		// Это числовой ID
		noteID = id
	} else {
		// Это название - декодируем из URL
		decodedTitle, err := url.QueryUnescape(parts[2])
		if err != nil {
			respondError(w, http.StatusBadRequest, "Invalid note title encoding")
			return
		}
		noteTitle = decodedTitle
	}

	// GET запрос - получить заметку
	if r.Method == http.MethodGet && len(parts) == 3 {
		var note NoteDetail
		var tagsArray []string
		var isPublic bool

		var query string
		var scanErr error

		if noteID > 0 {
			// Поиск по ID
			query = `
				SELECT n.id, n.title, n.content, n.tags, n.created_at, n.updated_at,
					   COALESCE(n.is_public, false),
					   COALESCE(un.favorite, false), COALESCE(un.read, false),
					   un.read_at, un.favorited_at
				FROM notes n
				LEFT JOIN user_notes un ON n.id = un.note_id AND un.user_id = $1
				WHERE n.id = $2
			`
			scanErr = db.QueryRow(query, userID, noteID).Scan(
				&note.ID, &note.Title, &note.Content,
				(*StringArray)(&tagsArray), &note.CreatedAt, &note.UpdatedAt, &isPublic,
				&note.Favorite, &note.Read, &note.ReadAt, &note.FavoritedAt,
			)
		} else {
			// Поиск по названию (регистронезависимый)
			query = `
				SELECT n.id, n.title, n.content, n.tags, n.created_at, n.updated_at,
					   COALESCE(n.is_public, false),
					   COALESCE(un.favorite, false), COALESCE(un.read, false),
					   un.read_at, un.favorited_at
				FROM notes n
				LEFT JOIN user_notes un ON n.id = un.note_id AND un.user_id = $1
				WHERE LOWER(n.title) = LOWER($2)
			`
			scanErr = db.QueryRow(query, userID, noteTitle).Scan(
				&note.ID, &note.Title, &note.Content,
				(*StringArray)(&tagsArray), &note.CreatedAt, &note.UpdatedAt, &isPublic,
				&note.Favorite, &note.Read, &note.ReadAt, &note.FavoritedAt,
			)
		}

		if scanErr == sql.ErrNoRows {
			respondError(w, http.StatusNotFound, "Note not found")
			return
		}
		if scanErr != nil {
			respondError(w, http.StatusInternalServerError, "Database error")
			return
		}

		// Проверяем доступ для гостей
		if role == "guest" && !isPublic {
			respondError(w, http.StatusForbidden, "Access denied")
			return
		}

		note.Tags = tagsArray
		respondJSON(w, http.StatusOK, note)
		return
	}

	// POST запрос - действия (favorite, unfavorite, read)
	if r.Method == http.MethodPost && len(parts) == 4 {
		action := parts[3]

		// Если это название, сначала получаем ID
		if noteID == 0 {
			scanErr := db.QueryRow(`SELECT id FROM notes WHERE LOWER(title) = LOWER($1)`, noteTitle).Scan(&noteID)
			if scanErr == sql.ErrNoRows {
				respondError(w, http.StatusNotFound, "Note not found")
				return
			}
			if scanErr != nil {
				respondError(w, http.StatusInternalServerError, "Database error")
				return
			}
		}

		switch action {
		case "favorite":
			_, err = db.Exec(`
				INSERT INTO user_notes (user_id, note_id, favorite, favorited_at)
				VALUES ($1, $2, true, $3)
				ON CONFLICT (user_id, note_id) DO UPDATE SET
					favorite = true, favorited_at = $3
			`, userID, noteID, time.Now())

		case "unfavorite":
			_, err = db.Exec(`
				UPDATE user_notes SET favorite = false, favorited_at = NULL
				WHERE user_id = $1 AND note_id = $2
			`, userID, noteID)

		case "read":
			_, err = db.Exec(`
				INSERT INTO user_notes (user_id, note_id, read, read_at)
				VALUES ($1, $2, true, $3)
				ON CONFLICT (user_id, note_id) DO UPDATE SET
					read = true, read_at = $3
			`, userID, noteID, time.Now())

		default:
			respondError(w, http.StatusNotFound, "Unknown action")
			return
		}

		if err != nil {
			respondError(w, http.StatusInternalServerError, "Database error")
			return
		}

		// Возвращаем обновлённую заметку
		var note NoteDetail
		var tagsArray []string

		db.QueryRow(`
			SELECT n.id, n.title, n.content, n.tags, n.created_at, n.updated_at,
				   COALESCE(un.favorite, false), COALESCE(un.read, false),
				   un.read_at, un.favorited_at
			FROM notes n
			LEFT JOIN user_notes un ON n.id = un.note_id AND un.user_id = $1
			WHERE n.id = $2
		`, userID, noteID).Scan(&note.ID, &note.Title, &note.Content,
			(*StringArray)(&tagsArray), &note.CreatedAt, &note.UpdatedAt,
			&note.Favorite, &note.Read, &note.ReadAt, &note.FavoritedAt)

		note.Tags = tagsArray
		respondJSON(w, http.StatusOK, note)
		return
	}

	respondError(w, http.StatusNotFound, "Not found")
}

func adminHandleNotes(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		// Получить все заметки
		rows, err := db.Query(`
			SELECT id, title, content, tags, is_public, saved_to_migration, 
			       created_at, updated_at
			FROM notes 
			ORDER BY created_at DESC
		`)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Database error")
			return
		}
		defer rows.Close()

		notes := []map[string]interface{}{}
		for rows.Next() {
			var id int
			var title, content string
			var tagsArray []string
			var isPublic, savedToMigration bool
			var createdAt, updatedAt time.Time

			rows.Scan(&id, &title, &content, (*StringArray)(&tagsArray),
				&isPublic, &savedToMigration, &createdAt, &updatedAt)

			notes = append(notes, map[string]interface{}{
				"id":                 id,
				"title":              title,
				"content":            content,
				"tags":               tagsArray,
				"is_public":          isPublic,
				"saved_to_migration": savedToMigration,
				"created_at":         createdAt,
				"updated_at":         updatedAt,
			})
		}
		respondJSON(w, http.StatusOK, map[string]interface{}{"notes": notes})

	} else if r.Method == http.MethodPost {
		// Создать новую заметку
		var note struct {
			Title    string   `json:"title"`
			Content  string   `json:"content"`
			Tags     []string `json:"tags"`
			IsPublic bool     `json:"is_public"`
		}
		if err := json.NewDecoder(r.Body).Decode(&note); err != nil {
			respondError(w, http.StatusBadRequest, "Invalid request")
			return
		}

		// Проверка обязательных полей
		if note.Title == "" || note.Content == "" {
			respondError(w, http.StatusBadRequest, "Title and content are required")
			return
		}

		var id int
		err := db.QueryRow(`
			INSERT INTO notes (title, content, tags, is_public, saved_to_migration, created_at, updated_at)
			VALUES ($1, $2, $3, $4, false, NOW(), NOW())
			RETURNING id
		`, note.Title, note.Content, StringArray(note.Tags), note.IsPublic).Scan(&id)
		if err != nil {
			if strings.Contains(err.Error(), "duplicate key") {
				respondError(w, http.StatusConflict, "Note with this title already exists")
				return
			}
			respondError(w, http.StatusInternalServerError, "Database error")
			return
		}

		respondJSON(w, http.StatusOK, map[string]interface{}{"id": id})
	}
}

func adminUpdateNote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 4 {
		respondError(w, http.StatusBadRequest, "Invalid note ID")
		return
	}

	noteID, err := strconv.Atoi(parts[3])
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid note ID")
		return
	}

	// Проверяем, что заметка не сохранена в миграцию
	var savedToMigration bool
	err = db.QueryRow(`SELECT saved_to_migration FROM notes WHERE id = $1`, noteID).
		Scan(&savedToMigration)

	if err == sql.ErrNoRows {
		respondError(w, http.StatusNotFound, "Note not found")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}

	// if savedToMigration {
	// 	respondError(w, http.StatusForbidden, "Cannot edit notes saved to migration")
	// 	return
	// }

	var note struct {
		Title    string   `json:"title"`
		Content  string   `json:"content"`
		Tags     []string `json:"tags"`
		IsPublic bool     `json:"is_public"`
	}
	if err := json.NewDecoder(r.Body).Decode(&note); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request")
		return
	}

	if note.Title == "" || note.Content == "" {
		respondError(w, http.StatusBadRequest, "Title and content are required")
		return
	}

	_, err = db.Exec(`
		UPDATE notes 
		SET title = $1, content = $2, tags = $3, is_public = $4, updated_at = NOW(), saved_to_migration = $6
		WHERE id = $5
	`, note.Title, note.Content, StringArray(note.Tags), note.IsPublic, noteID, false)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			respondError(w, http.StatusConflict, "Note with this title already exists")
			return
		}
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"success": true})
}

func adminDeleteNote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 4 {
		respondError(w, http.StatusBadRequest, "Invalid note ID")
		return
	}

	noteID, err := strconv.Atoi(parts[3])
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid note ID")
		return
	}

	// Проверяем, что заметка не сохранена в миграцию
	var savedToMigration bool
	err = db.QueryRow(`SELECT saved_to_migration FROM notes WHERE id = $1`, noteID).
		Scan(&savedToMigration)

	if err == sql.ErrNoRows {
		respondError(w, http.StatusNotFound, "Note not found")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}

	if savedToMigration {
		respondError(w, http.StatusForbidden, "Cannot delete notes saved to migration")
		return
	}

	_, err = db.Exec(`DELETE FROM notes WHERE id = $1 AND saved_to_migration = false`, noteID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"success": true})
}

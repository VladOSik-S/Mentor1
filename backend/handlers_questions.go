package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// SM-2 алгоритм интервального повторения
func calculateNextReview(quality int, easinessFactor float64, interval, repetitions int) (float64, int, int) {
	// quality: 0-5 (0 = полный провал, 5 = идеально)

	// Обновляем easiness factor
	newEF := easinessFactor + (0.1 - float64(5-quality)*(0.08+float64(5-quality)*0.02))
	if newEF < 1.3 {
		newEF = 1.3
	}

	var newInterval int
	var newRepetitions int

	if quality < 3 {
		// Провал - начинаем заново
		newInterval = 0
		newRepetitions = 0
	} else {
		if repetitions == 0 {
			newInterval = 1
		} else if repetitions == 1 {
			newInterval = 6
		} else {
			newInterval = int(math.Round(float64(interval) * newEF))
		}
		newRepetitions = repetitions + 1
	}

	return newEF, newInterval, newRepetitions
}

// POST /api/questions/{id}/answer - ответить на вопрос
func answerQuestion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 3 {
		respondError(w, http.StatusBadRequest, "Invalid question ID")
		return
	}

	questionID, err := strconv.Atoi(parts[2])
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid question ID")
		return
	}

	userID := getUserID(r)

	var req struct {
		Quality int `json:"quality"` // 0-5
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request")
		return
	}

	if req.Quality < 0 || req.Quality > 5 {
		respondError(w, http.StatusBadRequest, "Quality must be 0-5")
		return
	}

	// Получаем текущий прогресс
	var easinessFactor float64
	var interval, repetitions int
	var exists bool

	err = db.QueryRow(`
		SELECT easiness_factor, interval, repetitions, true
		FROM user_questions
		WHERE user_id = $1 AND question_id = $2
	`, userID, questionID).Scan(&easinessFactor, &interval, &repetitions, &exists)

	if err == sql.ErrNoRows {
		// Первый раз отвечаем на вопрос
		easinessFactor = 2.5
		interval = 0
		repetitions = 0
		exists = false
	} else if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}

	// Вычисляем новые параметры
	newEF, newInterval, newRepetitions := calculateNextReview(req.Quality, easinessFactor, interval, repetitions)
	nextReviewDate := time.Now().AddDate(0, 0, newInterval)

	// Определяем статус
	status := "learning"
	if newRepetitions >= 5 {
		status = "mastered"
	} else if newRepetitions >= 2 {
		status = "reviewing"
	}

	if exists {
		_, err = db.Exec(`
			UPDATE user_questions
			SET easiness_factor = $1, interval = $2, repetitions = $3,
			    next_review_date = $4, last_reviewed_at = $5, status = $6
			WHERE user_id = $7 AND question_id = $8
		`, newEF, newInterval, newRepetitions, nextReviewDate, time.Now(), status, userID, questionID)
	} else {
		_, err = db.Exec(`
			INSERT INTO user_questions 
			(user_id, question_id, easiness_factor, interval, repetitions, next_review_date, last_reviewed_at, status)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`, userID, questionID, newEF, newInterval, newRepetitions, nextReviewDate, time.Now(), status)
	}

	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"next_review_date": nextReviewDate,
		"interval":         newInterval,
		"repetitions":      newRepetitions,
		"status":           status,
	})
}

// ADMIN: GET /api/admin/questions
func adminGetQuestions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	rows, err := db.Query(`
		SELECT id, question, answer, tags, difficulty, created_at, updated_at
		FROM questions
		ORDER BY created_at DESC
	`)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer rows.Close()

	questions := []map[string]interface{}{}
	for rows.Next() {
		var id, difficulty int
		var question, answer string
		var tagsArray []string
		var createdAt, updatedAt time.Time

		rows.Scan(&id, &question, &answer, (*StringArray)(&tagsArray), &difficulty, &createdAt, &updatedAt)

		questions = append(questions, map[string]interface{}{
			"id":         id,
			"question":   question,
			"answer":     answer,
			"tags":       tagsArray,
			"difficulty": difficulty,
			"created_at": createdAt,
			"updated_at": updatedAt,
		})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"questions": questions})
}

// ADMIN: POST /api/admin/questions
func adminCreateQuestion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req struct {
		Question   string   `json:"question"`
		Answer     string   `json:"answer"`
		Tags       []string `json:"tags"`
		Difficulty int      `json:"difficulty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request")
		return
	}

	if req.Question == "" || req.Answer == "" {
		respondError(w, http.StatusBadRequest, "Question and answer are required")
		return
	}

	if req.Difficulty < 1 || req.Difficulty > 3 {
		req.Difficulty = 1
	}

	var id int
	err := db.QueryRow(`
		INSERT INTO questions (question, answer, tags, difficulty)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`, req.Question, req.Answer, StringArray(req.Tags), req.Difficulty).Scan(&id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"id": id})
}

// ADMIN: PUT /api/admin/questions/{id}
func adminUpdateQuestion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 4 {
		respondError(w, http.StatusBadRequest, "Invalid question ID")
		return
	}

	questionID, err := strconv.Atoi(parts[3])
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid question ID")
		return
	}

	var req struct {
		Question   string   `json:"question"`
		Answer     string   `json:"answer"`
		Tags       []string `json:"tags"`
		Difficulty int      `json:"difficulty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request")
		return
	}

	if req.Question == "" || req.Answer == "" {
		respondError(w, http.StatusBadRequest, "Question and answer are required")
		return
	}

	_, err = db.Exec(`
		UPDATE questions
		SET question = $1, answer = $2, tags = $3, difficulty = $4, updated_at = NOW()
		WHERE id = $5
	`, req.Question, req.Answer, StringArray(req.Tags), req.Difficulty, questionID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"success": true})
}

// ADMIN: DELETE /api/admin/questions/{id}
func adminDeleteQuestion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 4 {
		respondError(w, http.StatusBadRequest, "Invalid question ID")
		return
	}

	questionID, err := strconv.Atoi(parts[3])
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid question ID")
		return
	}

	_, err = db.Exec(`DELETE FROM questions WHERE id = $1`, questionID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"success": true})
}

// GET /api/questions - получить вопросы для изучения
func getQuestions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	userID := getUserID(r)
	filter := r.URL.Query().Get("filter") // new, due, all
	tags := r.URL.Query()["tags"]

	// Получаем назначенные пользователю теги
	var assignedTags []string
	rows, err := db.Query(`SELECT tag FROM user_question_tags WHERE user_id = $1`, userID)
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

	// Если у пользователя нет назначенных тегов, возвращаем пустой список
	if len(assignedTags) == 0 {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"questions": []map[string]interface{}{},
			"stats":     map[string]int{"new": 0, "learning": 0, "reviewing": 0, "mastered": 0, "due": 0},
		})
		return
	}

	var query string
	var args []interface{}
	argCount := 0

	if filter == "due" {
		query = `
			SELECT q.id, q.question, q.answer, q.tags, q.difficulty,
			       uq.status, uq.next_review_date, uq.repetitions
			FROM questions q
			JOIN user_questions uq ON q.id = uq.question_id
			WHERE uq.user_id = $1 AND uq.next_review_date <= NOW()
			  AND q.tags && $2
		`
		args = append(args, userID, StringArray(assignedTags))
		argCount = 2
	} else if filter == "new" {
		query = `
			SELECT q.id, q.question, q.answer, q.tags, q.difficulty,
			       'new' as status, NULL as next_review_date, 0 as repetitions
			FROM questions q
			WHERE q.id NOT IN (
				SELECT question_id FROM user_questions WHERE user_id = $1
			) AND q.tags && $2
		`
		args = append(args, userID, StringArray(assignedTags))
		argCount = 2
	} else {
		query = `
			SELECT q.id, q.question, q.answer, q.tags, q.difficulty,
			       COALESCE(uq.status, 'new') as status,
			       uq.next_review_date, COALESCE(uq.repetitions, 0) as repetitions
			FROM questions q
			LEFT JOIN user_questions uq ON q.id = uq.question_id AND uq.user_id = $1
			WHERE q.tags && $2
		`
		args = append(args, userID, StringArray(assignedTags))
		argCount = 2
	}

	// Фильтр по выбранным тегам
	if len(tags) > 0 {
		argCount++
		query += fmt.Sprintf(` AND q.tags && $%d`, argCount)
		args = append(args, StringArray(tags))
	}

	query += ` ORDER BY q.id`

	questionsRows, err := db.Query(query, args...)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer questionsRows.Close()

	questions := []map[string]interface{}{}
	for questionsRows.Next() {
		var id, difficulty, repetitions int
		var question, answer, status string
		var tagsArray []string
		var nextReviewDate sql.NullTime

		questionsRows.Scan(&id, &question, &answer, (*StringArray)(&tagsArray), &difficulty,
			&status, &nextReviewDate, &repetitions)

		q := map[string]interface{}{
			"id":          id,
			"question":    question,
			"answer":      answer,
			"tags":        tagsArray,
			"difficulty":  difficulty,
			"status":      status,
			"repetitions": repetitions,
		}

		if nextReviewDate.Valid {
			q["next_review_date"] = nextReviewDate.Time
		}

		questions = append(questions, q)
	}

	// Подсчитываем статистику (только для назначенных тегов)
	var stats struct {
		New       int `json:"new"`
		Learning  int `json:"learning"`
		Reviewing int `json:"reviewing"`
		Mastered  int `json:"mastered"`
		Due       int `json:"due"`
	}

	db.QueryRow(`
		SELECT 
			COUNT(*) FILTER (WHERE uq.question_id IS NULL) as new,
			COUNT(*) FILTER (WHERE uq.status = 'learning') as learning,
			COUNT(*) FILTER (WHERE uq.status = 'reviewing') as reviewing,
			COUNT(*) FILTER (WHERE uq.status = 'mastered') as mastered,
			COUNT(*) FILTER (WHERE uq.next_review_date <= NOW()) as due
		FROM questions q
		LEFT JOIN user_questions uq ON q.id = uq.question_id AND uq.user_id = $1
		WHERE q.tags && $2
	`, userID, StringArray(assignedTags)).Scan(&stats.New, &stats.Learning, &stats.Reviewing, &stats.Mastered, &stats.Due)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"questions": questions,
		"stats":     stats,
	})
}

// GET /api/questions/tags - получить теги доступные пользователю
func getQuestionTags(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	userID := getUserID(r)

	// Получаем назначенные пользователю теги
	rows, err := db.Query(`SELECT tag FROM user_question_tags WHERE user_id = $1 ORDER BY tag`, userID)
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

package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func adminGetUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	rows, err := db.Query(`
		SELECT u.id, u.telegram_id, u.name, u.status, u.role,
			   COUNT(DISTINCT CASE WHEN us.status = 'completed' THEN us.sprint_id END) as completed_sprints,
			   COUNT(DISTINCT CASE WHEN ut.completed THEN ut.task_id END) as completed_tasks
		FROM users u
		LEFT JOIN user_sprints us ON u.id = us.user_id
		LEFT JOIN user_tasks ut ON u.id = ut.user_id
		GROUP BY u.id
		ORDER BY u.created_at DESC
	`)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer rows.Close()

	users := []map[string]interface{}{}
	for rows.Next() {
		var id int
		var telegramID, name, status, role string
		var completedSprints, completedTasks int
		rows.Scan(&id, &telegramID, &name, &status, &role, &completedSprints, &completedTasks)
		users = append(users, map[string]interface{}{
			"id": id, "telegram_id": telegramID, "name": name,
			"status": status, "role": role,
			"completed_sprints": completedSprints,
			"completed_tasks":   completedTasks,
		})
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"users": users})
}

func adminHandleUser(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 4 {
		respondError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	userID, err := strconv.Atoi(parts[3])
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	if r.Method == http.MethodGet {
		var user map[string]interface{}
		var name, status, role string
		err := db.QueryRow(`SELECT name, status, role FROM users WHERE id = $1`, userID).
			Scan(&name, &status, &role)
		if err != nil {
			respondError(w, http.StatusNotFound, "User not found")
			return
		}

		user = map[string]interface{}{"id": userID, "name": name, "status": status, "role": role}

		// Get assigned sprints
		rows, _ := db.Query(`
			SELECT s.id, s.name, us.status 
			FROM user_sprints us 
			JOIN sprints s ON us.sprint_id = s.id 
			WHERE us.user_id = $1
		`, userID)
		defer rows.Close()

		sprints := []map[string]interface{}{}
		for rows.Next() {
			var id int
			var name, status string
			rows.Scan(&id, &name, &status)
			sprints = append(sprints, map[string]interface{}{"id": id, "name": name, "status": status})
		}
		user["sprints"] = sprints

		respondJSON(w, http.StatusOK, user)
	}
}

func adminHandleSprints(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		rows, err := db.Query(`SELECT id, name, description, duration_days, order_index FROM sprints ORDER BY order_index`)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Database error")
			return
		}
		defer rows.Close()

		sprints := []map[string]interface{}{}
		for rows.Next() {
			var id, duration, orderIndex int
			var name, description string
			rows.Scan(&id, &name, &description, &duration, &orderIndex)
			sprints = append(sprints, map[string]interface{}{
				"id": id, "name": name, "description": description,
				"duration_days": duration, "order_index": orderIndex,
			})
		}
		respondJSON(w, http.StatusOK, map[string]interface{}{"sprints": sprints})
	} else if r.Method == http.MethodPost {
		var sprint struct {
			Name         string `json:"name"`
			Description  string `json:"description"`
			DurationDays int    `json:"duration_days"`
		}
		if err := json.NewDecoder(r.Body).Decode(&sprint); err != nil {
			respondError(w, http.StatusBadRequest, "Invalid request")
			return
		}

		var maxOrder int
		db.QueryRow(`SELECT COALESCE(MAX(order_index), 0) FROM sprints`).Scan(&maxOrder)

		var id int
		err := db.QueryRow(`
			INSERT INTO sprints (name, description, duration_days, order_index)
			VALUES ($1, $2, $3, $4) RETURNING id
		`, sprint.Name, sprint.Description, sprint.DurationDays, maxOrder+1).Scan(&id)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Database error")
			return
		}
		respondJSON(w, http.StatusOK, map[string]interface{}{"id": id})
	}
}

func adminHandleSprint(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 4 {
		respondError(w, http.StatusBadRequest, "Invalid sprint ID")
		return
	}

	sprintID, err := strconv.Atoi(parts[3])
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid sprint ID")
		return
	}

	if r.Method == http.MethodDelete {
		_, err := db.Exec(`DELETE FROM sprints WHERE id = $1`, sprintID)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Database error")
			return
		}
		respondJSON(w, http.StatusOK, map[string]interface{}{"success": true})
	}
}

func adminHandleTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		sprintID := r.URL.Query().Get("sprint_id")
		query := `SELECT id, sprint_id, name, description, type, content_url, time_minutes, order_index FROM tasks`
		var rows *sql.Rows
		var err error

		if sprintID != "" {
			query += ` WHERE sprint_id = $1 ORDER BY order_index`
			rows, err = db.Query(query, sprintID)
		} else {
			query += ` ORDER BY sprint_id, order_index`
			rows, err = db.Query(query)
		}

		if err != nil {
			respondError(w, http.StatusInternalServerError, "Database error")
			return
		}
		defer rows.Close()

		tasks := []map[string]interface{}{}
		for rows.Next() {
			var id, sprintID, timeMinutes, orderIndex int
			var name, description, taskType, contentURL string
			rows.Scan(&id, &sprintID, &name, &description, &taskType, &contentURL, &timeMinutes, &orderIndex)
			tasks = append(tasks, map[string]interface{}{
				"id": id, "sprint_id": sprintID, "name": name, "description": description,
				"type": taskType, "content_url": contentURL, "time_minutes": timeMinutes, "order_index": orderIndex,
			})
		}
		respondJSON(w, http.StatusOK, map[string]interface{}{"tasks": tasks})
	} else if r.Method == http.MethodPost {
		var task struct {
			SprintID    int    `json:"sprint_id"`
			Name        string `json:"name"`
			Description string `json:"description"`
			Type        string `json:"type"`
			Content     string `json:"content"`
			ContentURL  string `json:"content_url"`
			TimeMinutes int    `json:"time_minutes"`
		}
		if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
			respondError(w, http.StatusBadRequest, "Invalid request")
			return
		}

		var maxOrder int
		db.QueryRow(`SELECT COALESCE(MAX(order_index), 0) FROM tasks WHERE sprint_id = $1`, task.SprintID).Scan(&maxOrder)

		var id int
		err := db.QueryRow(`
            INSERT INTO tasks (sprint_id, name, description, type, content, content_url, time_minutes, order_index)
            VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id
        `, task.SprintID, task.Name, task.Description, task.Type, task.Content, task.ContentURL, task.TimeMinutes, maxOrder+1).Scan(&id)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Database error")
			return
		}
		respondJSON(w, http.StatusOK, map[string]interface{}{"id": id})
	}
}

func adminHandleTask(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 4 {
		respondError(w, http.StatusBadRequest, "Invalid task ID")
		return
	}

	taskID, err := strconv.Atoi(parts[3])
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid task ID")
		return
	}

	if r.Method == http.MethodDelete {
		_, err := db.Exec(`DELETE FROM tasks WHERE id = $1`, taskID)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Database error")
			return
		}
		respondJSON(w, http.StatusOK, map[string]interface{}{"success": true})
	}
}

func adminAssignSprint(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req struct {
		UserID   int    `json:"user_id"`
		SprintID int    `json:"sprint_id"`
		Status   string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request")
		return
	}

	_, err := db.Exec(`
		INSERT INTO user_sprints (user_id, sprint_id, status, started_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, sprint_id) DO UPDATE SET status = $3
	`, req.UserID, req.SprintID, req.Status, time.Now())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"success": true})
}

func adminAssignTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req struct {
		UserID int `json:"user_id"`
		TaskID int `json:"task_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request")
		return
	}

	_, err := db.Exec(`
		INSERT INTO user_tasks (user_id, task_id, completed)
		VALUES ($1, $2, false)
		ON CONFLICT (user_id, task_id) DO NOTHING
	`, req.UserID, req.TaskID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"success": true})
}

func adminGetSolutions(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	query := `
        SELECT s.id, s.task_id, s.user_id, s.content, s.status, s.created_at,
               u.name, t.name
        FROM task_solutions s
        JOIN users u ON s.user_id = u.id
        JOIN tasks t ON s.task_id = t.id
    `
	if status != "" {
		query += ` WHERE s.status = $1`
	}
	query += ` ORDER BY s.created_at DESC`

	var rows *sql.Rows
	var err error
	if status != "" {
		rows, err = db.Query(query, status)
	} else {
		rows, err = db.Query(query)
	}
	_ = err

	defer rows.Close()
	solutions := []map[string]interface{}{}
	for rows.Next() {
		var id, taskID, userID int
		var content, status, userName, taskName string
		var createdAt time.Time
		rows.Scan(&id, &taskID, &userID, &content, &status, &createdAt, &userName, &taskName)
		solutions = append(solutions, map[string]interface{}{
			"id": id, "task_id": taskID, "user_id": userID, "content": content,
			"status": status, "created_at": createdAt, "user_name": userName, "task_name": taskName,
		})
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"solutions": solutions})
}

func adminReviewSolution(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	solutionID, _ := strconv.Atoi(parts[3])

	var req struct {
		Status  string `json:"status"` // approved, rejected
		Comment string `json:"comment"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	_, err := db.Exec(`
        UPDATE task_solutions 
        SET status = $1, comment = $2, reviewed_at = $3, reviewer_id = $4
        WHERE id = $5
    `, req.Status, req.Comment, time.Now(), 1, solutionID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"success": true})
}

// Обновить спринт
func adminUpdateSprint(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 4 {
		respondError(w, http.StatusBadRequest, "Invalid sprint ID")
		return
	}

	sprintID, err := strconv.Atoi(parts[3])
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid sprint ID")
		return
	}

	var sprint struct {
		Name         string `json:"name"`
		Description  string `json:"description"`
		DurationDays int    `json:"duration_days"`
	}
	if err := json.NewDecoder(r.Body).Decode(&sprint); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request")
		return
	}

	_, err = db.Exec(`
		UPDATE sprints 
		SET name = $1, description = $2, duration_days = $3
		WHERE id = $4
	`, sprint.Name, sprint.Description, sprint.DurationDays, sprintID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"success": true})
}

// Обновить задачу
func adminUpdateTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 4 {
		respondError(w, http.StatusBadRequest, "Invalid task ID")
		return
	}

	taskID, err := strconv.Atoi(parts[3])
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid task ID")
		return
	}

	var task struct {
		SprintID    int    `json:"sprint_id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Type        string `json:"type"`
		Content     string `json:"content"`
		ContentURL  string `json:"content_url"`
		TimeMinutes int    `json:"time_minutes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request")
		return
	}

	_, err = db.Exec(`
		UPDATE tasks 
		SET sprint_id = $1, name = $2, description = $3, type = $4, 
		    content = $5, content_url = $6, time_minutes = $7
		WHERE id = $8
	`, task.SprintID, task.Name, task.Description, task.Type,
		task.Content, task.ContentURL, task.TimeMinutes, taskID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"success": true})
}

// Получить детальную информацию об ученике
func adminGetUserDetails(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 4 {
		respondError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	userID, err := strconv.Atoi(parts[3])
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	// Получаем информацию о пользователе
	var name, status, role string
	err = db.QueryRow(`SELECT name, status, role FROM users WHERE id = $1`, userID).
		Scan(&name, &status, &role)
	if err != nil {
		respondError(w, http.StatusNotFound, "User not found")
		return
	}

	user := map[string]interface{}{"id": userID, "name": name, "status": status, "role": role}

	// Получаем спринты ученика
	rows, _ := db.Query(`
		SELECT s.id, s.name, us.status,
		       COUNT(t.id) as tasks_count,
		       COUNT(CASE WHEN ut.completed THEN 1 END) as tasks_completed
		FROM user_sprints us 
		JOIN sprints s ON us.sprint_id = s.id 
		LEFT JOIN tasks t ON s.id = t.sprint_id
		LEFT JOIN user_tasks ut ON t.id = ut.task_id AND ut.user_id = $1
		WHERE us.user_id = $1
		GROUP BY s.id, s.name, us.status
	`, userID)
	defer rows.Close()

	sprints := []map[string]interface{}{}
	for rows.Next() {
		var id, tasksCount, tasksCompleted int
		var name, status string
		rows.Scan(&id, &name, &status, &tasksCount, &tasksCompleted)

		progress := 0
		if tasksCount > 0 {
			progress = (tasksCompleted * 100) / tasksCount
		}

		sprints = append(sprints, map[string]interface{}{
			"id": id, "name": name, "status": status, "progress": progress,
		})
	}

	// Получаем выполненные задачи
	taskRows, _ := db.Query(`
		SELECT t.id, t.name, ut.completed_at
		FROM user_tasks ut
		JOIN tasks t ON ut.task_id = t.id
		WHERE ut.user_id = $1 AND ut.completed = true
		ORDER BY ut.completed_at DESC
		LIMIT 20
	`, userID)
	defer taskRows.Close()

	tasks := []map[string]interface{}{}
	for taskRows.Next() {
		var id int
		var name string
		var completedAt time.Time
		taskRows.Scan(&id, &name, &completedAt)
		tasks = append(tasks, map[string]interface{}{
			"id": id, "name": name, "completed": true,
			"completed_at": completedAt.Format("2006-01-02"),
		})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"user":    user,
		"sprints": sprints,
		"tasks":   tasks,
	})
}

// Отозвать спринт у ученика
func adminUnassignSprint(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req struct {
		UserID   int `json:"user_id"`
		SprintID int `json:"sprint_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request")
		return
	}

	_, err := db.Exec(`DELETE FROM user_sprints WHERE user_id = $1 AND sprint_id = $2`,
		req.UserID, req.SprintID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"success": true})
}

// GET /api/admin/users/:id/question-tags
func adminGetUserQuestionTags(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 5 {
		respondError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	userID, err := strconv.Atoi(parts[3])
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	rows, err := db.Query(`
		SELECT tag, assigned_at 
		FROM user_question_tags 
		WHERE user_id = $1 
		ORDER BY tag
	`, userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer rows.Close()

	tags := []map[string]interface{}{}
	for rows.Next() {
		var tag string
		var assignedAt time.Time
		rows.Scan(&tag, &assignedAt)
		tags = append(tags, map[string]interface{}{
			"tag":         tag,
			"assigned_at": assignedAt,
		})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"tags": tags})
}

// POST /api/admin/users/:id/question-tags
func adminAssignQuestionTag(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 5 {
		respondError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	userID, err := strconv.Atoi(parts[3])
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	var req struct {
		Tag string `json:"tag"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request")
		return
	}

	adminID := getUserID(r)

	_, err = db.Exec(`
		INSERT INTO user_question_tags (user_id, tag, assigned_by, assigned_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (user_id, tag) DO NOTHING
	`, userID, req.Tag, adminID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"success": true})
}

// DELETE /api/admin/users/:id/question-tags/:tag
func adminUnassignQuestionTag(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 6 {
		respondError(w, http.StatusBadRequest, "Invalid parameters")
		return
	}

	userID, err := strconv.Atoi(parts[3])
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	tag := parts[5]

	_, err = db.Exec(`DELETE FROM user_question_tags WHERE user_id = $1 AND tag = $2`, userID, tag)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"success": true})
}

// GET /api/admin/users/:id/note-tags
func adminGetUserNoteTags(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 5 {
		respondError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	userID, err := strconv.Atoi(parts[3])
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	rows, err := db.Query(`
		SELECT tag, assigned_at 
		FROM user_note_tags 
		WHERE user_id = $1 
		ORDER BY tag
	`, userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer rows.Close()

	tags := []map[string]interface{}{}
	for rows.Next() {
		var tag string
		var assignedAt time.Time
		rows.Scan(&tag, &assignedAt)
		tags = append(tags, map[string]interface{}{
			"tag":         tag,
			"assigned_at": assignedAt,
		})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"tags": tags})
}

// POST /api/admin/users/:id/note-tags
func adminAssignNoteTag(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 5 {
		respondError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	userID, err := strconv.Atoi(parts[3])
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	var req struct {
		Tag string `json:"tag"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request")
		return
	}

	adminID := getUserID(r)

	_, err = db.Exec(`
		INSERT INTO user_note_tags (user_id, tag, assigned_by, assigned_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (user_id, tag) DO NOTHING
	`, userID, req.Tag, adminID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"success": true})
}

// DELETE /api/admin/users/:id/note-tags/:tag
func adminUnassignNoteTag(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 6 {
		respondError(w, http.StatusBadRequest, "Invalid parameters")
		return
	}

	userID, err := strconv.Atoi(parts[3])
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	tag := parts[5]

	_, err = db.Exec(`DELETE FROM user_note_tags WHERE user_id = $1 AND tag = $2`, userID, tag)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"success": true})
}

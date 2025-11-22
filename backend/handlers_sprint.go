package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

func getSprints(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	userID := getUserID(r)
	status := r.URL.Query().Get("status")

	query := `
        SELECT 
            s.id, s.name, 
            us.status,
            us.started_at, us.completed_at,
            COUNT(t.id) as tasks_count,
            COUNT(CASE WHEN ut.completed THEN 1 END) as tasks_completed
        FROM sprints s
        INNER JOIN user_sprints us ON s.id = us.sprint_id AND us.user_id = $1
        LEFT JOIN tasks t ON s.id = t.sprint_id
        LEFT JOIN user_tasks ut ON t.id = ut.task_id AND ut.user_id = $1
        WHERE 1=1
    `

	var args []interface{}
	args = append(args, userID)

	if status != "" && status != "all" {
		query += ` AND us.status = $2`
		args = append(args, status)
	}

	query += ` GROUP BY s.id, s.name, us.status, us.started_at, us.completed_at ORDER BY s.order_index`

	rows, err := db.Query(query, args...)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer rows.Close()

	sprints := []Sprint{}
	for rows.Next() {
		var s Sprint
		err := rows.Scan(&s.ID, &s.Name, &s.Status, &s.StartedAt, &s.CompletedAt, &s.TasksCount, &s.TasksCompleted)
		if err != nil {
			continue
		}

		if s.TasksCount > 0 {
			s.Progress = (s.TasksCompleted * 100) / s.TasksCount
		}

		sprints = append(sprints, s)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"sprints": sprints})
}

func handleSprint(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 3 {
		respondError(w, http.StatusBadRequest, "Invalid sprint ID")
		return
	}

	sprintID, err := strconv.Atoi(parts[2])
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid sprint ID")
		return
	}

	userID := getUserID(r)

	if len(parts) == 3 && r.Method == http.MethodGet {
		var sprint SprintDetail
		err := db.QueryRow(`
            SELECT s.id, s.name, s.description, s.duration_days,
                   COALESCE(us.status, 'locked'), us.started_at, us.completed_at
            FROM sprints s
            LEFT JOIN user_sprints us ON s.id = us.sprint_id AND us.user_id = $1
            WHERE s.id = $2
        `, userID, sprintID).Scan(&sprint.ID, &sprint.Name, &sprint.Description, &sprint.DurationDays,
			&sprint.Status, &sprint.StartedAt, &sprint.CompletedAt)

		if err == sql.ErrNoRows {
			respondError(w, http.StatusNotFound, "Sprint not found")
			return
		}
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Database error")
			return
		}

		db.QueryRow(`
            SELECT COUNT(*), COUNT(CASE WHEN ut.completed THEN 1 END)
            FROM tasks t
            LEFT JOIN user_tasks ut ON t.id = ut.task_id AND ut.user_id = $1
            WHERE t.sprint_id = $2
        `, userID, sprintID).Scan(&sprint.TasksCount, &sprint.TasksCompleted)

		if sprint.TasksCount > 0 {
			sprint.Progress = (sprint.TasksCompleted * 100) / sprint.TasksCount
		}

		respondJSON(w, http.StatusOK, sprint)
		return
	}

	if len(parts) == 4 && parts[3] == "tasks" && r.Method == http.MethodGet {
		rows, err := db.Query(`
            SELECT t.id, t.name, t.type, t.time_minutes, t.order_index,
                   COALESCE(ut.completed, false)
            FROM tasks t
            LEFT JOIN user_tasks ut ON t.id = ut.task_id AND ut.user_id = $1
            WHERE t.sprint_id = $2
            ORDER BY t.order_index
        `, userID, sprintID)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Database error")
			return
		}
		defer rows.Close()

		tasks := []Task{}
		for rows.Next() {
			var t Task
			rows.Scan(&t.ID, &t.Name, &t.Type, &t.TimeMinutes, &t.Order, &t.Completed)
			t.TimeFormatted = fmt.Sprintf("%d мин", t.TimeMinutes)
			tasks = append(tasks, t)
		}

		respondJSON(w, http.StatusOK, map[string]interface{}{"tasks": tasks})
		return
	}

	respondError(w, http.StatusNotFound, "Not found")
}

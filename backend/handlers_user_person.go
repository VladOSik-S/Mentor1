package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
)

func getUserMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	userID := getUserID(r)
	var user User
	err := db.QueryRow(`
        SELECT id, telegram_id, name, status, avatar_url, created_at, role
        FROM users WHERE id = $1
    `, userID).Scan(&user.ID, &user.TelegramID, &user.Name, &user.Status, &user.AvatarURL, &user.CreatedAt, &user.Role)

	if err == sql.ErrNoRows {
		respondError(w, http.StatusNotFound, "User not found")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}

	respondJSON(w, http.StatusOK, user)
}

func getUserStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	userID := getUserID(r)
	var stats UserStats

	err := db.QueryRow(`
        SELECT 
            COALESCE(SUM(t.time_minutes), 0) as total_time,
            COUNT(DISTINCT CASE WHEN ut.completed THEN ut.task_id END) as completed_tasks,
            COUNT(DISTINCT t.id) as total_tasks,
            COUNT(DISTINCT CASE WHEN us.status = 'completed' THEN us.sprint_id END) as completed_sprints
        FROM users u
        LEFT JOIN user_sprints us ON u.id = us.user_id
        LEFT JOIN tasks t ON us.sprint_id = t.sprint_id
        LEFT JOIN user_tasks ut ON u.id = ut.user_id AND t.id = ut.task_id
        WHERE u.id = $1
    `, userID).Scan(&stats.TotalTimeMinutes, &stats.TasksCompleted, &stats.TasksTotal, &stats.SprintsCompleted)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}

	hours := stats.TotalTimeMinutes / 60
	minutes := stats.TotalTimeMinutes % 60
	stats.TotalTimeFormatted = fmt.Sprintf("%dч %dм", hours, minutes)

	var currentSprint sql.NullInt64
	db.QueryRow(`
        SELECT sprint_id FROM user_sprints 
        WHERE user_id = $1 AND status = 'active'
        ORDER BY sprint_id LIMIT 1
    `, userID).Scan(&currentSprint)

	if currentSprint.Valid {
		stats.CurrentSprint = int(currentSprint.Int64)
	}

	respondJSON(w, http.StatusOK, stats)
}

func handleSettings(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)

	switch r.Method {
	case http.MethodGet:
		var settings UserSettings
		err := db.QueryRow(`
            SELECT theme, notifications_enabled 
            FROM user_settings WHERE user_id = $1
        `, userID).Scan(&settings.Theme, &settings.NotificationsEnabled)

		if err == sql.ErrNoRows {
			settings = UserSettings{Theme: "dark", NotificationsEnabled: true}
			_, err = db.Exec(`
                INSERT INTO user_settings (user_id, theme, notifications_enabled)
                VALUES ($1, $2, $3)
            `, userID, settings.Theme, settings.NotificationsEnabled)
		}

		if err != nil && err != sql.ErrNoRows {
			respondError(w, http.StatusInternalServerError, "Database error")
			return
		}

		respondJSON(w, http.StatusOK, settings)

	case http.MethodPatch:
		var update UserSettingsUpdate
		if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
			respondError(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		_, err := db.Exec(`
            INSERT INTO user_settings (user_id, theme, notifications_enabled)
            VALUES ($1, $2, $3)
            ON CONFLICT (user_id) DO UPDATE SET
                theme = COALESCE($2, user_settings.theme),
                notifications_enabled = COALESCE($3, user_settings.notifications_enabled)
        `, userID, update.Theme, update.NotificationsEnabled)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Database error")
			return
		}

		var settings UserSettings
		db.QueryRow(`SELECT theme, notifications_enabled FROM user_settings WHERE user_id = $1`, userID).
			Scan(&settings.Theme, &settings.NotificationsEnabled)

		respondJSON(w, http.StatusOK, settings)

	default:
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

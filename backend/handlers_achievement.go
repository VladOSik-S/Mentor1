package main

import (
	"net/http"
)

func getAchievements(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	userID := getUserID(r)

	rows, err := db.Query(`
        SELECT a.id, a.name, a.description, a.icon,
               COALESCE(ua.unlocked, false), COALESCE(ua.progress, 0), ua.unlocked_at
        FROM achievements a
        LEFT JOIN user_achievements ua ON a.id = ua.achievement_id AND ua.user_id = $1
        ORDER BY a.id
    `, userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer rows.Close()

	achievements := []Achievement{}
	for rows.Next() {
		var a Achievement
		rows.Scan(&a.ID, &a.Name, &a.Description, &a.Icon, &a.Unlocked, &a.Progress, &a.UnlockedAt)
		achievements = append(achievements, a)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"achievements": achievements})
}

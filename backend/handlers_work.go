package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// GET /api/work/stats - статистика откликов
func getWorkStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	userID := getUserID(r)

	// Статистика за сегодня
	var todayStats struct {
		TotalApplications   int `json:"total_applications"`
		DailyLimit          int `json:"daily_limit"`
		Remaining           int `json:"remaining"`
		ResponsesReceived   int `json:"responses_received"`
		InterviewsScheduled int `json:"interviews_scheduled"`
	}

	today := time.Now().Format("2006-01-02")
	err := db.QueryRow(`
		SELECT COALESCE(total_applications, 0), COALESCE(daily_limit, 200),
		       COALESCE(responses_received, 0), COALESCE(interviews_scheduled, 0)
		FROM applications_stats
		WHERE user_id = $1 AND date = $2
	`, userID, today).Scan(&todayStats.TotalApplications, &todayStats.DailyLimit,
		&todayStats.ResponsesReceived, &todayStats.InterviewsScheduled)

	if err == sql.ErrNoRows {
		todayStats.DailyLimit = 200
	}

	todayStats.Remaining = todayStats.DailyLimit - todayStats.TotalApplications
	if todayStats.Remaining < 0 {
		todayStats.Remaining = 0
	}

	// Статистика за неделю
	weekAgo := time.Now().AddDate(0, 0, -7).Format("2006-01-02")
	var weekStats struct {
		TotalApplications   int `json:"total_applications"`
		ResponsesReceived   int `json:"responses_received"`
		InterviewsScheduled int `json:"interviews_scheduled"`
	}

	db.QueryRow(`
		SELECT COALESCE(SUM(total_applications), 0),
		       COALESCE(SUM(responses_received), 0),
		       COALESCE(SUM(interviews_scheduled), 0)
		FROM applications_stats
		WHERE user_id = $1 AND date >= $2
	`, userID, weekAgo).Scan(&weekStats.TotalApplications,
		&weekStats.ResponsesReceived, &weekStats.InterviewsScheduled)

	// Общая статистика
	var totalStats struct {
		TotalApplications  int `json:"total_applications"`
		ActiveApplications int `json:"active_applications"`
		Rejections         int `json:"rejections"`
		Interviews         int `json:"interviews"`
		Offers             int `json:"offers"`
	}

	db.QueryRow(`
		SELECT COUNT(*),
		       COUNT(CASE WHEN status IN ('pending', 'viewed') THEN 1 END),
		       COUNT(CASE WHEN status = 'rejected' THEN 1 END),
		       COUNT(CASE WHEN status = 'interview' THEN 1 END),
		       COUNT(CASE WHEN status = 'offer' THEN 1 END)
		FROM job_applications
		WHERE user_id = $1
	`, userID).Scan(&totalStats.TotalApplications, &totalStats.ActiveApplications,
		&totalStats.Rejections, &totalStats.Interviews, &totalStats.Offers)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"today": todayStats,
		"week":  weekStats,
		"total": totalStats,
	})
}

// GET /api/work/cover-letters - получить сопроводительные
func getCoverLetters(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	userID := getUserID(r)

	rows, err := db.Query(`
		SELECT id, title, content, is_default, created_at
		FROM cover_letters
		WHERE user_id = $1
		ORDER BY is_default DESC, created_at DESC
	`, userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer rows.Close()

	letters := []map[string]interface{}{}
	for rows.Next() {
		var id int
		var title, content string
		var isDefault bool
		var createdAt time.Time

		rows.Scan(&id, &title, &content, &isDefault, &createdAt)
		letters = append(letters, map[string]interface{}{
			"id":         id,
			"title":      title,
			"content":    content,
			"is_default": isDefault,
			"created_at": createdAt,
		})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"letters": letters})
}

// POST /api/work/cover-letters - создать сопроводительное
func createCoverLetter(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	userID := getUserID(r)

	var req struct {
		Title     string `json:"title"`
		Content   string `json:"content"`
		IsDefault bool   `json:"is_default"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request")
		return
	}

	if req.Title == "" || req.Content == "" {
		respondError(w, http.StatusBadRequest, "Title and content are required")
		return
	}

	// Если это дефолтное, снимаем дефолт с остальных
	if req.IsDefault {
		db.Exec(`UPDATE cover_letters SET is_default = false WHERE user_id = $1`, userID)
	}

	var id int
	err := db.QueryRow(`
		INSERT INTO cover_letters (user_id, title, content, is_default)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`, userID, req.Title, req.Content, req.IsDefault).Scan(&id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"id": id})
}

// PUT /api/work/cover-letters/:id - обновить сопроводительное
func updateCoverLetter(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 4 {
		respondError(w, http.StatusBadRequest, "Invalid letter ID")
		return
	}

	letterID, err := strconv.Atoi(parts[3])
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid letter ID")
		return
	}

	userID := getUserID(r)

	var req struct {
		Title     string `json:"title"`
		Content   string `json:"content"`
		IsDefault bool   `json:"is_default"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request")
		return
	}

	// Если это дефолтное, снимаем дефолт с остальных
	if req.IsDefault {
		db.Exec(`UPDATE cover_letters SET is_default = false WHERE user_id = $1`, userID)
	}

	_, err = db.Exec(`
		UPDATE cover_letters
		SET title = $1, content = $2, is_default = $3, updated_at = NOW()
		WHERE id = $4 AND user_id = $5
	`, req.Title, req.Content, req.IsDefault, letterID, userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"success": true})
}

// DELETE /api/work/cover-letters/:id - удалить сопроводительное
func deleteCoverLetter(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 4 {
		respondError(w, http.StatusBadRequest, "Invalid letter ID")
		return
	}

	letterID, err := strconv.Atoi(parts[3])
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid letter ID")
		return
	}

	userID := getUserID(r)

	_, err = db.Exec(`DELETE FROM cover_letters WHERE id = $1 AND user_id = $2`, letterID, userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"success": true})
}

// GET /api/work/activity - получить лог действий
func getWorkActivity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	userID := getUserID(r)
	limit := r.URL.Query().Get("limit")
	if limit == "" {
		limit = "50"
	}

	rows, err := db.Query(`
		SELECT id, vacancy_id, vacancy_title, company_name, action_type, description, created_at
		FROM work_activity_log
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer rows.Close()

	activities := []map[string]interface{}{}
	for rows.Next() {
		var id int
		var vacancyID, vacancyTitle, companyName, actionType, description string
		var createdAt time.Time

		rows.Scan(&id, &vacancyID, &vacancyTitle, &companyName, &actionType, &description, &createdAt)
		activities = append(activities, map[string]interface{}{
			"id":            id,
			"vacancy_id":    vacancyID,
			"vacancy_title": vacancyTitle,
			"company_name":  companyName,
			"action_type":   actionType,
			"description":   description,
			"created_at":    createdAt,
		})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"activities": activities})
}

// POST /api/work/activity - добавить действие в лог
func addWorkActivity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	userID := getUserID(r)

	var req struct {
		VacancyID    string `json:"vacancy_id"`
		VacancyTitle string `json:"vacancy_title"`
		CompanyName  string `json:"company_name"`
		ActionType   string `json:"action_type"` // applied, hr_response, interview_scheduled, rejection, offer, custom
		Description  string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request")
		return
	}

	if req.ActionType == "" {
		respondError(w, http.StatusBadRequest, "Action type is required")
		return
	}

	var id int
	err := db.QueryRow(`
		INSERT INTO work_activity_log (user_id, vacancy_id, vacancy_title, company_name, action_type, description)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`, userID, req.VacancyID, req.VacancyTitle, req.CompanyName, req.ActionType, req.Description).Scan(&id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"id": id})
}

// GET /api/work/applications - получить отклики
func getApplications(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	userID := getUserID(r)
	status := r.URL.Query().Get("status")

	query := `
		SELECT vacancy_id, vacancy_title, company_name, salary_from, salary_to, 
		       salary_currency, vacancy_url, status, applied_at, last_status_update
		FROM job_applications
		WHERE user_id = $1
	`
	args := []interface{}{userID}

	if status != "" && status != "all" {
		query += ` AND status = $2`
		args = append(args, status)
	}

	query += ` ORDER BY applied_at DESC LIMIT 100`

	rows, err := db.Query(query, args...)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer rows.Close()

	applications := []map[string]interface{}{}
	for rows.Next() {
		var vacancyID, vacancyTitle, companyName, status, vacancyURL string
		var salaryFrom, salaryTo sql.NullInt64
		var salaryCurrency sql.NullString
		var appliedAt, lastStatusUpdate time.Time

		rows.Scan(&vacancyID, &vacancyTitle, &companyName, &salaryFrom, &salaryTo,
			&salaryCurrency, &vacancyURL, &status, &appliedAt, &lastStatusUpdate)

		app := map[string]interface{}{
			"vacancy_id":         vacancyID,
			"vacancy_title":      vacancyTitle,
			"company_name":       companyName,
			"vacancy_url":        vacancyURL,
			"status":             status,
			"applied_at":         appliedAt,
			"last_status_update": lastStatusUpdate,
		}

		if salaryFrom.Valid {
			app["salary_from"] = salaryFrom.Int64
		}
		if salaryTo.Valid {
			app["salary_to"] = salaryTo.Int64
		}
		if salaryCurrency.Valid {
			app["salary_currency"] = salaryCurrency.String
		}

		applications = append(applications, app)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"applications": applications})
}

// POST /api/work/increment-applications - увеличить счетчик откликов
func incrementApplications(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	userID := getUserID(r)
	today := time.Now().Format("2006-01-02")

	_, err := db.Exec(`
		INSERT INTO applications_stats (user_id, date, total_applications, daily_limit)
		VALUES ($1, $2, 1, 200)
		ON CONFLICT (user_id, date) DO UPDATE
		SET total_applications = applications_stats.total_applications + 1
	`, userID, today)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"success": true})
}

// Calendar event handlers (using your existing calendar.go code)
// HH.ru handlers
func handleHHSettings(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)

	if r.Method == http.MethodGet {
		var settings HHSettings
		err := db.QueryRow(`
            SELECT access_token, refresh_token, token_expires_at, resume_id, is_active
            FROM user_hh_settings WHERE user_id = $1
        `, userID).Scan(&settings.AccessToken, &settings.RefreshToken,
			&settings.TokenExpiresAt, &settings.ResumeID, &settings.IsActive)

		if err == sql.ErrNoRows {
			respondJSON(w, http.StatusOK, map[string]interface{}{"settings": nil})
			return
		}
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Database error")
			return
		}
		respondJSON(w, http.StatusOK, map[string]interface{}{"settings": settings})
	} else if r.Method == http.MethodPost {
		var settings HHSettings
		if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
			respondError(w, http.StatusBadRequest, "Invalid request")
			return
		}

		_, err := db.Exec(`
            INSERT INTO user_hh_settings (user_id, access_token, refresh_token, token_expires_at, resume_id, is_active)
            VALUES ($1, $2, $3, $4, $5, $6)
            ON CONFLICT (user_id) DO UPDATE 
            SET access_token = $2, refresh_token = $3, token_expires_at = $4, resume_id = $5, is_active = $6, updated_at = NOW()
        `, userID, settings.AccessToken, settings.RefreshToken, settings.TokenExpiresAt, settings.ResumeID, settings.IsActive)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Database error")
			return
		}
		respondJSON(w, http.StatusOK, map[string]interface{}{"success": true})
	}
}

func handleCoverLetters(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)

	if r.Method == http.MethodGet {
		rows, err := db.Query(`
            SELECT id, title, content, is_default, created_at
            FROM cover_letters WHERE user_id = $1 ORDER BY created_at DESC
        `, userID)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Database error")
			return
		}
		defer rows.Close()

		letters := []CoverLetter{}
		for rows.Next() {
			var letter CoverLetter
			rows.Scan(&letter.ID, &letter.Title, &letter.Content, &letter.IsDefault, &letter.CreatedAt)
			letters = append(letters, letter)
		}
		respondJSON(w, http.StatusOK, map[string]interface{}{"letters": letters})
	} else if r.Method == http.MethodPost {
		var letter CoverLetter
		if err := json.NewDecoder(r.Body).Decode(&letter); err != nil {
			respondError(w, http.StatusBadRequest, "Invalid request")
			return
		}

		var id int
		err := db.QueryRow(`
            INSERT INTO cover_letters (user_id, title, content, is_default)
            VALUES ($1, $2, $3, $4) RETURNING id
        `, userID, letter.Title, letter.Content, letter.IsDefault).Scan(&id)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Database error")
			return
		}
		respondJSON(w, http.StatusOK, map[string]interface{}{"id": id})
	}
}

func handleCoverLetter(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 4 {
		respondError(w, http.StatusBadRequest, "Invalid letter ID")
		return
	}

	letterID := parts[3]
	userID := getUserID(r)

	if r.Method == http.MethodPut {
		var letter CoverLetter
		if err := json.NewDecoder(r.Body).Decode(&letter); err != nil {
			respondError(w, http.StatusBadRequest, "Invalid request")
			return
		}

		_, err := db.Exec(`
            UPDATE cover_letters SET title = $1, content = $2, is_default = $3, updated_at = NOW()
            WHERE id = $4 AND user_id = $5
        `, letter.Title, letter.Content, letter.IsDefault, letterID, userID)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Database error")
			return
		}
		respondJSON(w, http.StatusOK, map[string]interface{}{"success": true})
	} else if r.Method == http.MethodDelete {
		_, err := db.Exec(`DELETE FROM cover_letters WHERE id = $1 AND user_id = $2`, letterID, userID)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Database error")
			return
		}
		respondJSON(w, http.StatusOK, map[string]interface{}{"success": true})
	}
}

func handleApplications(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)

	if r.Method == http.MethodGet {
		status := r.URL.Query().Get("status")
		query := `
            SELECT vacancy_id, vacancy_title, company_name, salary_from, salary_to, 
                   salary_currency, vacancy_url, status, applied_at
            FROM job_applications WHERE user_id = $1
        `
		args := []interface{}{userID}

		if status != "" {
			query += " AND status = $2"
			args = append(args, status)
		}
		query += " ORDER BY applied_at DESC"

		rows, err := db.Query(query, args...)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Database error")
			return
		}
		defer rows.Close()

		applications := []JobApplication{}
		for rows.Next() {
			var app JobApplication
			var vacancyURL, currency sql.NullString
			rows.Scan(&app.VacancyID, &app.VacancyTitle, &app.CompanyName, &app.SalaryFrom,
				&app.SalaryTo, &currency, &vacancyURL, &app.Status, &app.AppliedAt)
			applications = append(applications, app)
		}
		respondJSON(w, http.StatusOK, map[string]interface{}{"applications": applications})
	}
}

func getApplicationStats(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)

	rows, err := db.Query(`
        SELECT date, total_applications, daily_limit, responses_received, interviews_scheduled
        FROM applications_stats WHERE user_id = $1 ORDER BY date DESC LIMIT 30
    `, userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer rows.Close()

	stats := []map[string]interface{}{}
	for rows.Next() {
		var date time.Time
		var total, limit, responses, interviews int
		rows.Scan(&date, &total, &limit, &responses, &interviews)
		stats = append(stats, map[string]interface{}{
			"date":                 date.Format("2006-01-02"),
			"total_applications":   total,
			"daily_limit":          limit,
			"responses_received":   responses,
			"interviews_scheduled": interviews,
		})
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"stats": stats})
}

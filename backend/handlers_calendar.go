package main

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type CalendarEvent struct {
	ID                 int        `json:"id"`
	Title              string     `json:"title"`
	Description        string     `json:"description"`
	EventType          string     `json:"event_type"`
	Location           string     `json:"location"`
	StartTime          time.Time  `json:"start_time"`
	EndTime            time.Time  `json:"end_time"`
	AllDay             bool       `json:"all_day"`
	IsRecurring        bool       `json:"is_recurring"`
	RecurrenceRule     string     `json:"recurrence_rule"`
	RecurrenceInterval int        `json:"recurrence_interval"`
	RecurrenceDays     []int      `json:"recurrence_days"`
	RecurrenceEndDate  *time.Time `json:"recurrence_end_date"`
	IsPublic           bool       `json:"is_public"`
	TargetRole         string     `json:"target_role"`
	CreatedBy          *int       `json:"created_by"`
	IsPersonal         bool       `json:"is_personal"`
	Color              string     `json:"color,omitempty"`
	ReminderMinutes    *int       `json:"reminder_minutes,omitempty"`
	Notes              string     `json:"notes,omitempty"`
	IsHidden           bool       `json:"is_hidden"`
}

// GET /api/calendar/events - получить события календаря
func getCalendarEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	userID := getUserID(r)
	startDate := r.URL.Query().Get("start")
	endDate := r.URL.Query().Get("end")
	eventType := r.URL.Query().Get("type")

	var role string
	db.QueryRow(`SELECT role FROM users WHERE id = $1`, userID).Scan(&role)

	log.Printf("User role: %s", role)

	events := []CalendarEvent{}

	// Получаем глобальные события
	query := `
		SELECT e.id, e.title, e.description, e.event_type, e.location,
		       e.start_time, e.end_time, e.all_day, e.is_recurring,
		       e.recurrence_rule, e.recurrence_interval, e.recurrence_days,
		       e.recurrence_end_date, e.is_public, e.target_role, e.created_by,
		       COALESCE(ue.is_hidden, false), ue.reminder_minutes, ue.notes
		FROM calendar_events e
		LEFT JOIN user_calendar_events ue ON e.id = ue.event_id AND ue.user_id = $1
		WHERE (e.is_public = true OR e.target_role IS NULL OR e.target_role = $2)
	`

	args := []interface{}{userID, role}
	argCount := 2

	if startDate != "" && endDate != "" {
		argCount++
		query += fmt.Sprintf(` AND e.start_time >= $%d`, argCount)
		args = append(args, startDate)
		argCount++
		query += fmt.Sprintf(` AND e.end_time <= $%d`, argCount)
		args = append(args, endDate)
	}

	if eventType != "" && eventType != "all" {
		argCount++
		query += fmt.Sprintf(` AND e.event_type = $%d`, argCount)
		args = append(args, eventType)
	}

	query += ` ORDER BY e.start_time`

	rows, err := db.Query(query, args...)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}

	log.Printf("Query: %s", query)
	log.Printf("Args: %v", args)
	log.Printf("Query error: %v", err)
	defer rows.Close()

	for rows.Next() {
		var e CalendarEvent
		var recurrenceDays sql.NullString
		err := rows.Scan(&e.ID, &e.Title, &e.Description, &e.EventType, &e.Location,
			&e.StartTime, &e.EndTime, &e.AllDay, &e.IsRecurring,
			&e.RecurrenceRule, &e.RecurrenceInterval, &recurrenceDays,
			&e.RecurrenceEndDate, &e.IsPublic, &e.TargetRole, &e.CreatedBy,
			&e.IsHidden, &e.ReminderMinutes, &e.Notes)
		if err != nil {
			log.Printf("Scan error: %v", err)
			continue
		}

		if recurrenceDays.Valid {
			// Парсим массив дней
			daysStr := strings.Trim(recurrenceDays.String, "{}")
			if daysStr != "" {
				dayStrs := strings.Split(daysStr, ",")
				for _, d := range dayStrs {
					day, _ := strconv.Atoi(d)
					e.RecurrenceDays = append(e.RecurrenceDays, day)
				}
			}
		}
		log.Printf("Event: id=%d, title=%s, isHidden=%v", e.ID, e.Title, e.IsHidden)

		e.IsPersonal = false
		if !e.IsHidden {
			events = append(events, e)
		}
	}

	// Получаем личные события пользователя
	personalQuery := `
		SELECT id, title, description, location, start_time, end_time, all_day,
		       is_recurring, recurrence_rule, recurrence_interval, recurrence_days,
		       recurrence_end_date, reminder_minutes, color
		FROM personal_calendar_events
		WHERE user_id = $1
	`

	pArgs := []interface{}{userID}
	pArgCount := 1

	if startDate != "" && endDate != "" {
		pArgCount++
		personalQuery += fmt.Sprintf(` AND start_time >= $%d`, pArgCount)
		pArgs = append(pArgs, startDate)
		pArgCount++
		personalQuery += fmt.Sprintf(` AND end_time <= $%d`, pArgCount)
		pArgs = append(pArgs, endDate)
	}

	personalQuery += ` ORDER BY start_time`

	pRows, err := db.Query(personalQuery, pArgs...)
	if err == nil {
		defer pRows.Close()
		for pRows.Next() {
			var e CalendarEvent
			var recurrenceDays sql.NullString
			err := pRows.Scan(&e.ID, &e.Title, &e.Description, &e.Location,
				&e.StartTime, &e.EndTime, &e.AllDay, &e.IsRecurring,
				&e.RecurrenceRule, &e.RecurrenceInterval, &recurrenceDays,
				&e.RecurrenceEndDate, &e.ReminderMinutes, &e.Color)
			if err != nil {
				continue
			}

			if recurrenceDays.Valid {
				daysStr := strings.Trim(recurrenceDays.String, "{}")
				if daysStr != "" {
					dayStrs := strings.Split(daysStr, ",")
					for _, d := range dayStrs {
						day, _ := strconv.Atoi(d)
						e.RecurrenceDays = append(e.RecurrenceDays, day)
					}
				}
			}

			e.IsPersonal = true
			e.EventType = "personal"
			events = append(events, e)
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"events": events})
}

// POST /api/calendar/events - создать событие (личное)
func createPersonalEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	userID := getUserID(r)

	var req struct {
		Title              string     `json:"title"`
		Description        string     `json:"description"`
		Location           string     `json:"location"`
		StartTime          time.Time  `json:"start_time"`
		EndTime            time.Time  `json:"end_time"`
		AllDay             bool       `json:"all_day"`
		IsRecurring        bool       `json:"is_recurring"`
		RecurrenceRule     string     `json:"recurrence_rule"`
		RecurrenceInterval int        `json:"recurrence_interval"`
		RecurrenceDays     []int      `json:"recurrence_days"`
		RecurrenceEndDate  *time.Time `json:"recurrence_end_date"`
		ReminderMinutes    *int       `json:"reminder_minutes"`
		Color              string     `json:"color"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request")
		return
	}

	if req.Title == "" {
		respondError(w, http.StatusBadRequest, "Title is required")
		return
	}

	var id int
	err := db.QueryRow(`
		INSERT INTO personal_calendar_events 
		(user_id, title, description, location, start_time, end_time, all_day,
		 is_recurring, recurrence_rule, recurrence_interval, recurrence_days,
		 recurrence_end_date, reminder_minutes, color)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING id
	`, userID, req.Title, req.Description, req.Location, req.StartTime, req.EndTime,
		req.AllDay, req.IsRecurring, req.RecurrenceRule, req.RecurrenceInterval,
		IntArray(req.RecurrenceDays), req.RecurrenceEndDate, req.ReminderMinutes, req.Color).Scan(&id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"id": id})
}

// PUT /api/calendar/events/:id - обновить личное событие
func updatePersonalEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 4 {
		respondError(w, http.StatusBadRequest, "Invalid event ID")
		return
	}

	eventID, err := strconv.Atoi(parts[3])
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid event ID")
		return
	}

	userID := getUserID(r)

	var req struct {
		Title              string     `json:"title"`
		Description        string     `json:"description"`
		Location           string     `json:"location"`
		StartTime          time.Time  `json:"start_time"`
		EndTime            time.Time  `json:"end_time"`
		AllDay             bool       `json:"all_day"`
		IsRecurring        bool       `json:"is_recurring"`
		RecurrenceRule     string     `json:"recurrence_rule"`
		RecurrenceInterval int        `json:"recurrence_interval"`
		RecurrenceDays     []int      `json:"recurrence_days"`
		RecurrenceEndDate  *time.Time `json:"recurrence_end_date"`
		ReminderMinutes    *int       `json:"reminder_minutes"`
		Color              string     `json:"color"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request")
		return
	}

	_, err = db.Exec(`
		UPDATE personal_calendar_events
		SET title = $1, description = $2, location = $3, start_time = $4, end_time = $5,
		    all_day = $6, is_recurring = $7, recurrence_rule = $8, recurrence_interval = $9,
		    recurrence_days = $10, recurrence_end_date = $11, reminder_minutes = $12,
		    color = $13, updated_at = NOW()
		WHERE id = $14 AND user_id = $15
	`, req.Title, req.Description, req.Location, req.StartTime, req.EndTime,
		req.AllDay, req.IsRecurring, req.RecurrenceRule, req.RecurrenceInterval,
		IntArray(req.RecurrenceDays), req.RecurrenceEndDate, req.ReminderMinutes,
		req.Color, eventID, userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"success": true})
}

// DELETE /api/calendar/events/:id - удалить личное событие
func deletePersonalEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 4 {
		respondError(w, http.StatusBadRequest, "Invalid event ID")
		return
	}

	eventID, err := strconv.Atoi(parts[3])
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid event ID")
		return
	}

	userID := getUserID(r)

	_, err = db.Exec(`DELETE FROM personal_calendar_events WHERE id = $1 AND user_id = $2`,
		eventID, userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"success": true})
}

// POST /api/calendar/events/:id/hide - скрыть глобальное событие
func hideGlobalEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 4 {
		respondError(w, http.StatusBadRequest, "Invalid event ID")
		return
	}

	eventID, err := strconv.Atoi(parts[3])
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid event ID")
		return
	}

	userID := getUserID(r)

	_, err = db.Exec(`
		INSERT INTO user_calendar_events (user_id, event_id, is_hidden)
		VALUES ($1, $2, true)
		ON CONFLICT (user_id, event_id) DO UPDATE SET is_hidden = true
	`, userID, eventID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"success": true})
}

// ADMIN: GET /api/admin/calendar/events
func adminGetCalendarEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	rows, err := db.Query(`
		SELECT id, title, description, event_type, location, start_time, end_time,
		       all_day, is_recurring, recurrence_rule, recurrence_interval,
		       recurrence_days, recurrence_end_date, is_public, target_role, created_by
		FROM calendar_events
		ORDER BY start_time DESC
	`)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer rows.Close()

	events := []map[string]interface{}{}
	for rows.Next() {
		var id int
		var title, description, eventType, location, recurrenceRule, targetRole string
		var startTime, endTime time.Time
		var allDay, isRecurring, isPublic bool
		var recurrenceInterval int
		var recurrenceDays sql.NullString
		var recurrenceEndDate sql.NullTime
		var createdBy sql.NullInt64

		rows.Scan(&id, &title, &description, &eventType, &location, &startTime, &endTime,
			&allDay, &isRecurring, &recurrenceRule, &recurrenceInterval, &recurrenceDays,
			&recurrenceEndDate, &isPublic, &targetRole, &createdBy)

		event := map[string]interface{}{
			"id":                  id,
			"title":               title,
			"description":         description,
			"event_type":          eventType,
			"location":            location,
			"start_time":          startTime,
			"end_time":            endTime,
			"all_day":             allDay,
			"is_recurring":        isRecurring,
			"recurrence_rule":     recurrenceRule,
			"recurrence_interval": recurrenceInterval,
			"is_public":           isPublic,
			"target_role":         targetRole,
		}

		if recurrenceDays.Valid {
			event["recurrence_days"] = recurrenceDays.String
		}
		if recurrenceEndDate.Valid {
			event["recurrence_end_date"] = recurrenceEndDate.Time
		}
		if createdBy.Valid {
			event["created_by"] = createdBy.Int64
		}

		events = append(events, event)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"events": events})
}

// ADMIN: POST /api/admin/calendar/events
func adminCreateCalendarEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req struct {
		Title              string     `json:"title"`
		Description        string     `json:"description"`
		EventType          string     `json:"event_type"`
		Location           string     `json:"location"`
		StartTime          time.Time  `json:"start_time"`
		EndTime            time.Time  `json:"end_time"`
		AllDay             bool       `json:"all_day"`
		IsRecurring        bool       `json:"is_recurring"`
		RecurrenceRule     string     `json:"recurrence_rule"`
		RecurrenceInterval int        `json:"recurrence_interval"`
		RecurrenceDays     []int      `json:"recurrence_days"`
		RecurrenceEndDate  *time.Time `json:"recurrence_end_date"`
		IsPublic           bool       `json:"is_public"`
		TargetRole         string     `json:"target_role"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request")
		return
	}

	if req.Title == "" {
		respondError(w, http.StatusBadRequest, "Title is required")
		return
	}

	userID := getUserID(r)

	var id int
	err := db.QueryRow(`
		INSERT INTO calendar_events
		(title, description, event_type, location, start_time, end_time, all_day,
		 is_recurring, recurrence_rule, recurrence_interval, recurrence_days,
		 recurrence_end_date, is_public, target_role, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		RETURNING id
	`, req.Title, req.Description, req.EventType, req.Location, req.StartTime, req.EndTime,
		req.AllDay, req.IsRecurring, req.RecurrenceRule, req.RecurrenceInterval,
		IntArray(req.RecurrenceDays), req.RecurrenceEndDate, req.IsPublic, req.TargetRole, userID).Scan(&id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"id": id})
}

// ADMIN: PUT /api/admin/calendar/events/:id
func adminUpdateCalendarEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 5 {
		respondError(w, http.StatusBadRequest, "Invalid event ID")
		return
	}

	eventID, err := strconv.Atoi(parts[4])
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid event ID")
		return
	}

	var req struct {
		Title              string     `json:"title"`
		Description        string     `json:"description"`
		EventType          string     `json:"event_type"`
		Location           string     `json:"location"`
		StartTime          time.Time  `json:"start_time"`
		EndTime            time.Time  `json:"end_time"`
		AllDay             bool       `json:"all_day"`
		IsRecurring        bool       `json:"is_recurring"`
		RecurrenceRule     string     `json:"recurrence_rule"`
		RecurrenceInterval int        `json:"recurrence_interval"`
		RecurrenceDays     []int      `json:"recurrence_days"`
		RecurrenceEndDate  *time.Time `json:"recurrence_end_date"`
		IsPublic           bool       `json:"is_public"`
		TargetRole         string     `json:"target_role"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request")
		return
	}

	_, err = db.Exec(`
		UPDATE calendar_events
		SET title = $1, description = $2, event_type = $3, location = $4,
		    start_time = $5, end_time = $6, all_day = $7, is_recurring = $8,
		    recurrence_rule = $9, recurrence_interval = $10, recurrence_days = $11,
		    recurrence_end_date = $12, is_public = $13, target_role = $14,
		    updated_at = NOW()
		WHERE id = $15
	`, req.Title, req.Description, req.EventType, req.Location, req.StartTime, req.EndTime,
		req.AllDay, req.IsRecurring, req.RecurrenceRule, req.RecurrenceInterval,
		IntArray(req.RecurrenceDays), req.RecurrenceEndDate, req.IsPublic, req.TargetRole, eventID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"success": true})
}

// ADMIN: DELETE /api/admin/calendar/events/:id
func adminDeleteCalendarEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 5 {
		respondError(w, http.StatusBadRequest, "Invalid event ID")
		return
	}

	eventID, err := strconv.Atoi(parts[4])
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid event ID")
		return
	}

	_, err = db.Exec(`DELETE FROM calendar_events WHERE id = $1`, eventID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"success": true})
}

// IntArray - helper для работы с массивами int в PostgreSQL
type IntArray []int

func (a IntArray) Value() (driver.Value, error) {
	if len(a) == 0 {
		return "{}", nil
	}
	strs := make([]string, len(a))
	for i, v := range a {
		strs[i] = strconv.Itoa(v)
	}
	return "{" + strings.Join(strs, ",") + "}", nil
}

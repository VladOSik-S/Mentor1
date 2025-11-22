package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func handleTask(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 3 {
		respondError(w, http.StatusBadRequest, "Invalid task ID")
		return
	}

	taskID, err := strconv.Atoi(parts[2])
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid task ID")
		return
	}

	userID := getUserID(r)

	if len(parts) == 3 && r.Method == http.MethodGet {
		var task TaskDetail
		err := db.QueryRow(`
            SELECT t.id, t.name, t.description, t.type, t.content, t.content_url, 
                   t.time_minutes, t.order_index, t.sprint_id, 
                   COALESCE(ut.completed, false), ut.completed_at
            FROM tasks t
            LEFT JOIN user_tasks ut ON t.id = ut.task_id AND ut.user_id = $1
            WHERE t.id = $2
        `, userID, taskID).Scan(&task.ID, &task.Name, &task.Description, &task.Type,
			&task.Content, &task.ContentURL, &task.TimeMinutes, &task.Order, &task.SprintID,
			&task.Completed, &task.CompletedAt)

		if err == sql.ErrNoRows {
			respondError(w, http.StatusNotFound, "Task not found")
			return
		}
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Database error")
			return
		}

		task.TimeFormatted = fmt.Sprintf("%d мин", task.TimeMinutes)
		respondJSON(w, http.StatusOK, task)
		return
	}

	if len(parts) == 4 && r.Method == http.MethodPost {
		action := parts[3]
		completed := action == "complete"

		// Получаем sprint_id задачи
		var sprintID int
		err := db.QueryRow(`SELECT sprint_id FROM tasks WHERE id = $1`, taskID).Scan(&sprintID)
		if err != nil {
			log.Printf("Failed to get sprint_id for task %d: %v", taskID, err)
			respondError(w, http.StatusInternalServerError, "Database error")
			return
		}

		_, err = db.Exec(`
            INSERT INTO user_tasks (user_id, task_id, completed, completed_at)
            VALUES ($1, $2, $3, $4)
            ON CONFLICT (user_id, task_id) DO UPDATE SET
                completed = $3,
                completed_at = $4
        `, userID, taskID, completed, time.Now())

		if err != nil {
			log.Printf("Failed to update task completion: %v", err)
			respondError(w, http.StatusInternalServerError, "Database error")
			return
		}

		// Проверяем завершение спринта при отметке задачи как выполненной
		if completed && sprintID > 0 {
			if err := checkAndCompleteSprint(userID, sprintID); err != nil {
				log.Printf("Error checking sprint completion: %v", err)
				// Не возвращаем ошибку клиенту, так как задача уже отмечена
			}
		}

		var task TaskDetail
		db.QueryRow(`
            SELECT t.id, t.name, t.description, t.type, t.content_url, t.time_minutes,
                   t.order_index, t.sprint_id, ut.completed, ut.completed_at
            FROM tasks t
            JOIN user_tasks ut ON t.id = ut.task_id
            WHERE t.id = $1 AND ut.user_id = $2
        `, taskID, userID).Scan(&task.ID, &task.Name, &task.Description, &task.Type,
			&task.ContentURL, &task.TimeMinutes, &task.Order, &task.SprintID,
			&task.Completed, &task.CompletedAt)

		task.TimeFormatted = fmt.Sprintf("%d мин", task.TimeMinutes)
		respondJSON(w, http.StatusOK, task)
		return
	}

	respondError(w, http.StatusNotFound, "Not found")
}

func handleTaskSolutions(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	taskID, _ := strconv.Atoi(parts[2])
	userID := getUserID(r)

	if r.Method == http.MethodGet {
		rows, _ := db.Query(`
            SELECT id, content, status, created_at, comment
            FROM task_solutions 
            WHERE task_id = $1 AND user_id = $2
            ORDER BY created_at DESC
        `, taskID, userID)
		defer rows.Close()

		solutions := []TaskSolution{}
		for rows.Next() {
			var s TaskSolution
			rows.Scan(&s.ID, &s.Content, &s.Status, &s.CreatedAt, &s.Comment)
			solutions = append(solutions, s)
		}

		respondJSON(w, http.StatusOK, map[string]interface{}{"solutions": solutions})
	} else if r.Method == http.MethodPost {
		var req struct {
			Content string `json:"content"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		var id int
		db.QueryRow(`
            INSERT INTO task_solutions (task_id, user_id, content, status)
            VALUES ($1, $2, $3, 'pending') RETURNING id
        `, taskID, userID, req.Content).Scan(&id)

		respondJSON(w, http.StatusOK, map[string]interface{}{"id": id})
	}
}

// checkAndCompleteSprint проверяет завершение спринта и выполняет необходимые действия
func checkAndCompleteSprint(userID, sprintID int) error {
	// Проверяем, все ли задачи спринта выполнены
	var totalTasks, completedTasks int
	err := db.QueryRow(`
        SELECT 
            COUNT(t.id) as total,
            COUNT(CASE WHEN ut.completed = true THEN 1 END) as completed
        FROM tasks t
        LEFT JOIN user_tasks ut ON t.id = ut.task_id AND ut.user_id = $1
        WHERE t.sprint_id = $2
    `, userID, sprintID).Scan(&totalTasks, &completedTasks)

	if err != nil {
		log.Printf("Error checking sprint completion: %v", err)
		return err
	}

	log.Printf("Sprint %d progress for user %d: %d/%d tasks completed",
		sprintID, userID, completedTasks, totalTasks)

	// Если все задачи выполнены
	if totalTasks > 0 && totalTasks == completedTasks {
		// Проверяем текущий статус спринта
		var currentStatus string
		err := db.QueryRow(`
            SELECT status FROM user_sprints 
            WHERE user_id = $1 AND sprint_id = $2
        `, userID, sprintID).Scan(&currentStatus)

		if err != nil && err != sql.ErrNoRows {
			log.Printf("Error getting sprint status: %v", err)
			return err
		}

		// Если записи нет или статус не completed
		if err == sql.ErrNoRows || currentStatus != "completed" {
			// Используем UPSERT для безопасности
			_, err = db.Exec(`
                INSERT INTO user_sprints (user_id, sprint_id, status, started_at, completed_at)
                VALUES ($1, $2, 'completed', NOW(), NOW())
                ON CONFLICT (user_id, sprint_id) DO UPDATE 
                SET status = 'completed', completed_at = NOW()
                WHERE user_sprints.status != 'completed'
            `, userID, sprintID)

			if err != nil {
				log.Printf("Error updating sprint status: %v", err)
				return err
			}

			log.Printf("Sprint %d marked as completed for user %d", sprintID, userID)

			// Отправляем уведомление в Telegram (в горутине, чтобы не блокировать)
			go sendSprintCompletionNotification(userID, sprintID)

			// Автоматически назначаем следующий спринт
			go assignNextSprint(userID, sprintID)
		} else {
			log.Printf("Sprint %d already completed for user %d", sprintID, userID)
		}
	}

	return nil
}

// sendSprintCompletionNotification отправляет уведомление о завершении спринта в Telegram
func sendSprintCompletionNotification(userID, sprintID int) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Panic in sendSprintCompletionNotification: %v", r)
		}
	}()

	// Получаем данные пользователя и спринта
	var telegramID, userName, sprintName string
	err := db.QueryRow(`
        SELECT u.telegram_id, u.name, s.name
        FROM users u
        CROSS JOIN sprints s
        WHERE u.id = $1 AND s.id = $2
    `, userID, sprintID).Scan(&telegramID, &userName, &sprintName)

	if err != nil {
		log.Printf("Failed to get user/sprint data for notification: %v", err)
		return
	}

	// Проверяем, что есть telegram_id
	if telegramID == "" {
		log.Printf("User %d has no telegram_id set", userID)
		return
	}

	// Получаем токен бота
	botToken := getEnv("TELEGRAM_BOT_TOKEN", "")
	if botToken == "" {
		log.Println("TELEGRAM_BOT_TOKEN not set, skipping notification")
		return
	}

	// Формируем сообщение
	message := fmt.Sprintf(
		"🎉 *Поздравляем, %s!*\n\n"+
			"Вы успешно завершили спринт:\n"+
			"📚 *%s*\n\n"+
			"Следующий спринт уже назначен. Продолжайте обучение! 💪\n\n"+
			"Откройте приложение, чтобы начать новый спринт.",
		userName,
		sprintName,
	)

	// Отправляем через Telegram Bot API
	err = sendTelegramMessage(botToken, telegramID, message)
	if err != nil {
		log.Printf("Failed to send telegram notification: %v", err)
	} else {
		log.Printf("Successfully sent sprint completion notification to user %d (telegram: %s)",
			userID, telegramID)
	}
}

// sendTelegramMessage отправляет сообщение через Telegram Bot API
func sendTelegramMessage(botToken, chatID, text string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)

	payload := map[string]interface{}{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "Markdown",
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal telegram payload: %w", err)
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to send telegram request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram API error (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// assignNextSprint автоматически назначает следующий спринт пользователю
func assignNextSprint(userID, currentSprintID int) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Panic in assignNextSprint: %v", r)
		}
	}()

	// Находим следующий спринт по order_index
	var nextSprintID int
	var nextSprintName string
	err := db.QueryRow(`
        SELECT s.id, s.name
        FROM sprints s
        WHERE s.order_index > (
            SELECT order_index FROM sprints WHERE id = $1
        )
        AND s.id NOT IN (
            SELECT sprint_id FROM user_sprints WHERE user_id = $2
        )
        ORDER BY s.order_index ASC
        LIMIT 1
    `, currentSprintID, userID).Scan(&nextSprintID, &nextSprintName)

	if err == sql.ErrNoRows {
		log.Printf("No more sprints to assign for user %d after sprint %d", userID, currentSprintID)

		// Отправляем сообщение о завершении всех спринтов
		var telegramID, userName string
		db.QueryRow(`SELECT telegram_id, name FROM users WHERE id = $1`, userID).
			Scan(&telegramID, &userName)

		if telegramID != "" {
			botToken := getEnv("TELEGRAM_BOT_TOKEN", "")
			if botToken != "" {
				message := fmt.Sprintf(
					"🏆 *Невероятно, %s!*\n\n"+
						"Вы завершили *ВСЕ* доступные спринты!\n\n"+
						"Это грандиозное достижение. Обратитесь к ментору за следующими шагами в вашем обучении. 🚀",
					userName,
				)
				sendTelegramMessage(botToken, telegramID, message)
			}
		}
		return
	}

	if err != nil {
		log.Printf("Failed to find next sprint for user %d: %v", userID, err)
		return
	}

	// Назначаем следующий спринт
	_, err = db.Exec(`
        INSERT INTO user_sprints (user_id, sprint_id, status, started_at)
        VALUES ($1, $2, 'active', NOW())
        ON CONFLICT (user_id, sprint_id) DO NOTHING
    `, userID, nextSprintID)

	if err != nil {
		log.Printf("Failed to assign next sprint %d to user %d: %v", nextSprintID, userID, err)
		return
	}

	log.Printf("Successfully assigned sprint %d (%s) to user %d", nextSprintID, nextSprintName, userID)
}

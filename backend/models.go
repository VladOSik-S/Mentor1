package main

import (
	"database/sql/driver"
	"strings"
	"time"
)

type HHSettings struct {
	AccessToken    string    `json:"access_token"`
	RefreshToken   string    `json:"refresh_token"`
	TokenExpiresAt time.Time `json:"token_expires_at"`
	ResumeID       string    `json:"resume_id"`
	IsActive       bool      `json:"is_active"`
}

type CoverLetter struct {
	ID        int       `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	IsDefault bool      `json:"is_default"`
	CreatedAt time.Time `json:"created_at"`
}

type JobApplication struct {
	VacancyID    string    `json:"vacancy_id"`
	VacancyTitle string    `json:"vacancy_title"`
	CompanyName  string    `json:"company_name"`
	SalaryFrom   *int      `json:"salary_from"`
	SalaryTo     *int      `json:"salary_to"`
	Status       string    `json:"status"`
	AppliedAt    time.Time `json:"applied_at"`
}

type User struct {
	ID         int       `json:"id"`
	TelegramID string    `json:"telegram_id"`
	Name       string    `json:"name"`
	Status     string    `json:"status"`
	AvatarURL  *string   `json:"avatar_url"`
	CreatedAt  time.Time `json:"created_at"`
	Role       string    `json:"role"`
}

type UserStats struct {
	TotalTimeMinutes   int    `json:"total_time_minutes"`
	TotalTimeFormatted string `json:"total_time_formatted"`
	CurrentSprint      int    `json:"current_sprint"`
	TasksCompleted     int    `json:"tasks_completed"`
	TasksTotal         int    `json:"tasks_total"`
	SprintsCompleted   int    `json:"sprints_completed"`
}

type UserSettings struct {
	Theme                string `json:"theme"`
	NotificationsEnabled bool   `json:"notifications_enabled"`
}

type UserSettingsUpdate struct {
	Theme                *string `json:"theme,omitempty"`
	NotificationsEnabled *bool   `json:"notifications_enabled,omitempty"`
}

type Sprint struct {
	ID             int        `json:"id"`
	Name           string     `json:"name"`
	Status         string     `json:"status"`
	Progress       int        `json:"progress"`
	TasksCount     int        `json:"tasks_count"`
	TasksCompleted int        `json:"tasks_completed"`
	StartedAt      *time.Time `json:"started_at"`
	CompletedAt    *time.Time `json:"completed_at"`
}

type SprintDetail struct {
	Sprint
	Description  string `json:"description"`
	DurationDays int    `json:"duration_days"`
}

type Task struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	Type          string `json:"type"`
	TimeMinutes   int    `json:"time_minutes"`
	TimeFormatted string `json:"time_formatted"`
	Completed     bool   `json:"completed"`
	Order         int    `json:"order"`
}

type TaskDetail struct {
	Task
	Content     string     `json:"content"`
	Description string     `json:"description"`
	ContentURL  string     `json:"content_url"`
	SprintID    int        `json:"sprint_id"`
	CompletedAt *time.Time `json:"completed_at"`
}

type TaskSolution struct {
	ID         int        `json:"id"`
	TaskID     int        `json:"task_id"`
	UserID     int        `json:"user_id"`
	Content    string     `json:"content"`
	Status     string     `json:"status"` // pending, approved, rejected
	CreatedAt  time.Time  `json:"created_at"`
	ReviewedAt *time.Time `json:"reviewed_at"`
	ReviewerID *int       `json:"reviewer_id"`
	Comment    string     `json:"comment"`
}

type AuthRequest struct {
	TelegramID string `json:"telegram_id"`
	Name       string `json:"name"`
}

type AuthResponse struct {
	Token   string `json:"token"`
	User    User   `json:"user"`
	IsGuest bool   `json:"is_guest"`
}

type Note struct {
	ID        int       `json:"id"`
	Title     string    `json:"title"`
	Tags      []string  `json:"tags"`
	Favorite  bool      `json:"favorite"`
	Read      bool      `json:"read"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type NoteDetail struct {
	Note
	Content     string     `json:"content"`
	ReadAt      *time.Time `json:"read_at"`
	FavoritedAt *time.Time `json:"favorited_at"`
}

type Achievement struct {
	ID          int        `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Icon        string     `json:"icon"`
	Unlocked    bool       `json:"unlocked"`
	Progress    int        `json:"progress"`
	UnlockedAt  *time.Time `json:"unlocked_at"`
}

type StringArray []string

func (a *StringArray) Scan(src interface{}) error {
	if src == nil {
		*a = []string{}
		return nil
	}
	switch v := src.(type) {
	case []byte:
		str := string(v)
		str = strings.Trim(str, "{}")
		if str == "" {
			*a = []string{}
			return nil
		}
		*a = strings.Split(str, ",")
	case string:
		str := strings.Trim(v, "{}")
		if str == "" {
			*a = []string{}
			return nil
		}
		*a = strings.Split(str, ",")
	}
	return nil
}

func (a StringArray) Value() (driver.Value, error) {
	if len(a) == 0 {
		return "{}", nil
	}
	return "{" + strings.Join(a, ",") + "}", nil
}

package domain

import "time"

// NotificationStatus represents the delivery state of a notification.
type NotificationStatus string

const (
	StatusPending NotificationStatus = "pending"
	StatusSent    NotificationStatus = "sent"
	StatusFailed  NotificationStatus = "failed"
)

// Notification represents a dispatched notification record.
type Notification struct {
	ID           string
	UserID       string
	Channel      string
	TemplateName string
	TemplateVars map[string]string
	Status       NotificationStatus
	CreatedAt    time.Time
}

// Template represents a notification template stored in the database.
type Template struct {
	ID        string
	Name      string
	Channel   string
	Subject   string
	Body      string
	CreatedAt time.Time
}

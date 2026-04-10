package handlers

import (
	"time"

	taskdomain "example.com/taskservice/internal/domain/task"
)

type taskMutationDTO struct {
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Status      taskdomain.Status `json:"status"`
	Recurrence  *recurrenceDTO    `json:"recurrence,omitempty"`
}

type recurrenceDTO struct {
	Type          taskdomain.RecurrenceType `json:"type"`
	Every         *int                      `json:"every,omitempty"`
	Day           *int                      `json:"day,omitempty"`
	SpecificDates []string                  `json:"specific_dates,omitempty"`
	StartDate     string                    `json:"start_date,omitempty"`
	EndDate       *string                   `json:"end_date,omitempty"`
}

type taskDTO struct {
	ID           int64             `json:"id"`
	Title        string            `json:"title"`
	Description  string            `json:"description"`
	Status       taskdomain.Status `json:"status"`
	TemplateID   *int64            `json:"template_id,omitempty"`
	ScheduledFor *string           `json:"scheduled_for,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

func newTaskDTO(task *taskdomain.Task) taskDTO {
	var scheduledFor *string
	if task.ScheduledFor != nil {
		formatted := task.ScheduledFor.UTC().Format("2006-01-02")
		scheduledFor = &formatted
	}

	return taskDTO{
		ID:           task.ID,
		Title:        task.Title,
		Description:  task.Description,
		Status:       task.Status,
		TemplateID:   task.TemplateID,
		ScheduledFor: scheduledFor,
		CreatedAt:    task.CreatedAt,
		UpdatedAt:    task.UpdatedAt,
	}
}

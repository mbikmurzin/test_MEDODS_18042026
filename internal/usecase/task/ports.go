package task

import (
	"context"
	"time"

	taskdomain "example.com/taskservice/internal/domain/task"
)

type Repository interface {
	Create(ctx context.Context, task *taskdomain.Task) (*taskdomain.Task, error)
	GetByID(ctx context.Context, id int64) (*taskdomain.Task, error)
	Update(ctx context.Context, task *taskdomain.Task) (*taskdomain.Task, error)
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context) ([]taskdomain.Task, error)
	CreateTemplate(ctx context.Context, template *taskdomain.TaskTemplate) (*taskdomain.TaskTemplate, error)
	GetTemplateByID(ctx context.Context, id int64) (*taskdomain.TaskTemplate, error)
	UpdateTemplate(ctx context.Context, template *taskdomain.TaskTemplate) (*taskdomain.TaskTemplate, error)
	DeleteTemplate(ctx context.Context, id int64) error
	ListTemplates(ctx context.Context) ([]taskdomain.TaskTemplate, error)
	UpsertGeneratedTask(ctx context.Context, task *taskdomain.Task) (*taskdomain.Task, error)
	DeleteGeneratedFromDate(ctx context.Context, templateID int64, fromDate time.Time) error
}

type Usecase interface {
	Create(ctx context.Context, input CreateInput) (*taskdomain.Task, error)
	GetByID(ctx context.Context, id int64) (*taskdomain.Task, error)
	Update(ctx context.Context, id int64, input UpdateInput) (*taskdomain.Task, error)
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context) ([]taskdomain.Task, error)
}

type CreateInput struct {
	Title       string
	Description string
	Status      taskdomain.Status
	Recurrence  *RecurrenceInput
}

type UpdateInput struct {
	Title       string
	Description string
	Status      taskdomain.Status
	Recurrence  *RecurrenceInput
}

type RecurrenceInput struct {
	Type          taskdomain.RecurrenceType
	Every         *int
	Day           *int
	SpecificDates []string
	StartDate     string
	EndDate       *string
}

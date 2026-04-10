package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	taskdomain "example.com/taskservice/internal/domain/task"
)

const recurrenceDateLayout = "2006-01-02"

type Repository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Create(ctx context.Context, task *taskdomain.Task) (*taskdomain.Task, error) {
	const query = `
		INSERT INTO tasks (title, description, status, template_id, scheduled_for, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, title, description, status, template_id, scheduled_for, created_at, updated_at
	`

	row := r.pool.QueryRow(
		ctx,
		query,
		task.Title,
		task.Description,
		task.Status,
		task.TemplateID,
		task.ScheduledFor,
		task.CreatedAt,
		task.UpdatedAt,
	)

	created, err := scanTask(row)
	if err != nil {
		return nil, err
	}

	return created, nil
}

func (r *Repository) GetByID(ctx context.Context, id int64) (*taskdomain.Task, error) {
	const query = `
		SELECT id, title, description, status, template_id, scheduled_for, created_at, updated_at
		FROM tasks
		WHERE id = $1
	`

	row := r.pool.QueryRow(ctx, query, id)
	found, err := scanTask(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, taskdomain.ErrNotFound
		}

		return nil, err
	}

	return found, nil
}

func (r *Repository) Update(ctx context.Context, task *taskdomain.Task) (*taskdomain.Task, error) {
	const query = `
		UPDATE tasks
		SET title = $1,
			description = $2,
			status = $3,
			updated_at = $4
		WHERE id = $5
		RETURNING id, title, description, status, template_id, scheduled_for, created_at, updated_at
	`

	row := r.pool.QueryRow(ctx, query, task.Title, task.Description, task.Status, task.UpdatedAt, task.ID)
	updated, err := scanTask(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, taskdomain.ErrNotFound
		}

		return nil, err
	}

	return updated, nil
}

func (r *Repository) Delete(ctx context.Context, id int64) error {
	const query = `DELETE FROM tasks WHERE id = $1`

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return taskdomain.ErrNotFound
	}

	return nil
}

func (r *Repository) List(ctx context.Context) ([]taskdomain.Task, error) {
	const query = `
		SELECT id, title, description, status, template_id, scheduled_for, created_at, updated_at
		FROM tasks
		ORDER BY id DESC
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tasks := make([]taskdomain.Task, 0)
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, err
		}

		tasks = append(tasks, *task)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return tasks, nil
}

func (r *Repository) CreateTemplate(ctx context.Context, template *taskdomain.TaskTemplate) (*taskdomain.TaskTemplate, error) {
	const query = `
		INSERT INTO task_templates (
			title, description, status, recurrence_type, every_n_days, monthly_day, specific_dates,
			start_date, end_date, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8, $9, $10, $11)
		RETURNING id, title, description, status, recurrence_type, every_n_days, monthly_day, specific_dates, start_date, end_date, created_at, updated_at
	`

	specificDatesJSON, err := recurrenceDatesToJSON(template.Recurrence.SpecificDates)
	if err != nil {
		return nil, err
	}

	row := r.pool.QueryRow(
		ctx,
		query,
		template.Title,
		template.Description,
		template.Status,
		template.Recurrence.Type,
		nullInt(template.Recurrence.EveryNDays),
		nullInt(template.Recurrence.MonthlyDay),
		specificDatesJSON,
		template.Recurrence.StartDate,
		template.Recurrence.EndDate,
		template.CreatedAt,
		template.UpdatedAt,
	)

	created, err := scanTemplate(row)
	if err != nil {
		return nil, err
	}

	return created, nil
}

func (r *Repository) GetTemplateByID(ctx context.Context, id int64) (*taskdomain.TaskTemplate, error) {
	const query = `
		SELECT id, title, description, status, recurrence_type, every_n_days, monthly_day, specific_dates, start_date, end_date, created_at, updated_at
		FROM task_templates
		WHERE id = $1
	`

	row := r.pool.QueryRow(ctx, query, id)
	template, err := scanTemplate(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, taskdomain.ErrNotFound
		}

		return nil, err
	}

	return template, nil
}

func (r *Repository) UpdateTemplate(ctx context.Context, template *taskdomain.TaskTemplate) (*taskdomain.TaskTemplate, error) {
	const query = `
		UPDATE task_templates
		SET title = $1,
			description = $2,
			status = $3,
			recurrence_type = $4,
			every_n_days = $5,
			monthly_day = $6,
			specific_dates = $7::jsonb,
			start_date = $8,
			end_date = $9,
			updated_at = $10
		WHERE id = $11
		RETURNING id, title, description, status, recurrence_type, every_n_days, monthly_day, specific_dates, start_date, end_date, created_at, updated_at
	`

	specificDatesJSON, err := recurrenceDatesToJSON(template.Recurrence.SpecificDates)
	if err != nil {
		return nil, err
	}

	row := r.pool.QueryRow(
		ctx,
		query,
		template.Title,
		template.Description,
		template.Status,
		template.Recurrence.Type,
		nullInt(template.Recurrence.EveryNDays),
		nullInt(template.Recurrence.MonthlyDay),
		specificDatesJSON,
		template.Recurrence.StartDate,
		template.Recurrence.EndDate,
		template.UpdatedAt,
		template.ID,
	)

	updated, err := scanTemplate(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, taskdomain.ErrNotFound
		}

		return nil, err
	}

	return updated, nil
}

func (r *Repository) DeleteTemplate(ctx context.Context, id int64) error {
	const query = `DELETE FROM task_templates WHERE id = $1`

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return taskdomain.ErrNotFound
	}

	return nil
}

func (r *Repository) ListTemplates(ctx context.Context) ([]taskdomain.TaskTemplate, error) {
	const query = `
		SELECT id, title, description, status, recurrence_type, every_n_days, monthly_day, specific_dates, start_date, end_date, created_at, updated_at
		FROM task_templates
		ORDER BY id ASC
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	templates := make([]taskdomain.TaskTemplate, 0)
	for rows.Next() {
		template, scanErr := scanTemplate(rows)
		if scanErr != nil {
			return nil, scanErr
		}

		templates = append(templates, *template)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return templates, nil
}

func (r *Repository) UpsertGeneratedTask(ctx context.Context, task *taskdomain.Task) (*taskdomain.Task, error) {
	const query = `
		INSERT INTO tasks (title, description, status, template_id, scheduled_for, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (template_id, scheduled_for)
		DO UPDATE SET
			title = EXCLUDED.title,
			description = EXCLUDED.description,
			status = EXCLUDED.status,
			updated_at = EXCLUDED.updated_at
		RETURNING id, title, description, status, template_id, scheduled_for, created_at, updated_at
	`

	row := r.pool.QueryRow(
		ctx,
		query,
		task.Title,
		task.Description,
		task.Status,
		task.TemplateID,
		task.ScheduledFor,
		task.CreatedAt,
		task.UpdatedAt,
	)

	upserted, err := scanTask(row)
	if err != nil {
		return nil, err
	}

	return upserted, nil
}

func (r *Repository) DeleteGeneratedFromDate(ctx context.Context, templateID int64, fromDate time.Time) error {
	const query = `
		DELETE FROM tasks
		WHERE template_id = $1
		  AND scheduled_for >= $2
	`

	_, err := r.pool.Exec(ctx, query, templateID, fromDate)
	return err
}

type taskScanner interface {
	Scan(dest ...any) error
}

func scanTask(scanner taskScanner) (*taskdomain.Task, error) {
	var (
		task         taskdomain.Task
		status       string
		templateID   sql.NullInt64
		scheduledFor sql.NullTime
	)

	if err := scanner.Scan(
		&task.ID,
		&task.Title,
		&task.Description,
		&status,
		&templateID,
		&scheduledFor,
		&task.CreatedAt,
		&task.UpdatedAt,
	); err != nil {
		return nil, err
	}

	task.Status = taskdomain.Status(status)
	if templateID.Valid {
		value := templateID.Int64
		task.TemplateID = &value
	}
	if scheduledFor.Valid {
		value := startOfDayUTC(scheduledFor.Time)
		task.ScheduledFor = &value
	}

	return &task, nil
}

func scanTemplate(scanner taskScanner) (*taskdomain.TaskTemplate, error) {
	var (
		template      taskdomain.TaskTemplate
		status        string
		recurrence    string
		everyNDays    sql.NullInt64
		monthlyDay    sql.NullInt64
		specificDates []byte
		startDate     time.Time
		endDate       sql.NullTime
	)

	if err := scanner.Scan(
		&template.ID,
		&template.Title,
		&template.Description,
		&status,
		&recurrence,
		&everyNDays,
		&monthlyDay,
		&specificDates,
		&startDate,
		&endDate,
		&template.CreatedAt,
		&template.UpdatedAt,
	); err != nil {
		return nil, err
	}

	recurrenceType := taskdomain.RecurrenceType(recurrence)
	if !recurrenceType.Valid() {
		return nil, fmt.Errorf("invalid recurrence type in db: %s", recurrence)
	}

	parsedSpecificDates, err := recurrenceDatesFromJSON(specificDates)
	if err != nil {
		return nil, err
	}

	template.Status = taskdomain.Status(status)
	template.Recurrence.Type = recurrenceType
	template.Recurrence.StartDate = startOfDayUTC(startDate)
	template.Recurrence.SpecificDates = parsedSpecificDates
	if everyNDays.Valid {
		template.Recurrence.EveryNDays = int(everyNDays.Int64)
	}
	if monthlyDay.Valid {
		template.Recurrence.MonthlyDay = int(monthlyDay.Int64)
	}
	if endDate.Valid {
		value := startOfDayUTC(endDate.Time)
		template.Recurrence.EndDate = &value
	}

	return &template, nil
}

func recurrenceDatesToJSON(dates []time.Time) ([]byte, error) {
	if len(dates) == 0 {
		return []byte("[]"), nil
	}

	values := make([]string, 0, len(dates))
	for _, date := range dates {
		values = append(values, startOfDayUTC(date).Format(recurrenceDateLayout))
	}

	encoded, err := json.Marshal(values)
	if err != nil {
		return nil, err
	}

	return encoded, nil
}

func recurrenceDatesFromJSON(raw []byte) ([]time.Time, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, err
	}

	parsedDates := make([]time.Time, 0, len(values))
	for _, value := range values {
		parsedDate, err := time.Parse(recurrenceDateLayout, value)
		if err != nil {
			return nil, err
		}
		parsedDates = append(parsedDates, startOfDayUTC(parsedDate))
	}

	return parsedDates, nil
}

func nullInt(value int) any {
	if value <= 0 {
		return nil
	}

	return value
}

func startOfDayUTC(value time.Time) time.Time {
	y, m, d := value.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

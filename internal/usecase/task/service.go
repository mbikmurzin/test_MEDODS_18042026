package task

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	taskdomain "example.com/taskservice/internal/domain/task"
)

const (
	recurrenceDateLayout = "2006-01-02"
	generationHorizon    = 30
)

type Service struct {
	repo Repository
	now  func() time.Time
}

func NewService(repo Repository) *Service {
	return &Service{
		repo: repo,
		now:  func() time.Time { return time.Now().UTC() },
	}
}

func (s *Service) Create(ctx context.Context, input CreateInput) (*taskdomain.Task, error) {
	normalized, recurrence, err := validateCreateInput(input)
	if err != nil {
		return nil, err
	}

	if recurrence == nil {
		return s.createRegularTask(ctx, normalized)
	}

	if err := ensureOccurrencesInHorizon(*recurrence, s.now()); err != nil {
		return nil, err
	}

	now := s.now().UTC()
	template := &taskdomain.TaskTemplate{
		Title:       normalized.Title,
		Description: normalized.Description,
		Status:      normalized.Status,
		Recurrence:  *recurrence,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	createdTemplate, err := s.repo.CreateTemplate(ctx, template)
	if err != nil {
		return nil, err
	}

	generated, err := s.generateForTemplate(ctx, createdTemplate, true)
	if err != nil {
		return nil, err
	}

	return &generated[0], nil
}

func (s *Service) createRegularTask(ctx context.Context, input CreateInput) (*taskdomain.Task, error) {
	model := &taskdomain.Task{
		Title:       input.Title,
		Description: input.Description,
		Status:      input.Status,
	}
	now := s.now().UTC()
	model.CreatedAt = now
	model.UpdatedAt = now

	created, err := s.repo.Create(ctx, model)
	if err != nil {
		return nil, err
	}

	return created, nil
}

func (s *Service) GetByID(ctx context.Context, id int64) (*taskdomain.Task, error) {
	if id <= 0 {
		return nil, fmt.Errorf("%w: id must be positive", ErrInvalidInput)
	}

	return s.repo.GetByID(ctx, id)
}

func (s *Service) Update(ctx context.Context, id int64, input UpdateInput) (*taskdomain.Task, error) {
	if id <= 0 {
		return nil, fmt.Errorf("%w: id must be positive", ErrInvalidInput)
	}

	normalized, recurrence, err := validateUpdateInput(input)
	if err != nil {
		return nil, err
	}

	existingTask, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if existingTask.TemplateID == nil {
		if recurrence == nil {
			model := &taskdomain.Task{
				ID:          id,
				Title:       normalized.Title,
				Description: normalized.Description,
				Status:      normalized.Status,
				UpdatedAt:   s.now().UTC(),
			}

			updated, updateErr := s.repo.Update(ctx, model)
			if updateErr != nil {
				return nil, updateErr
			}

			return updated, nil
		}

		now := s.now().UTC()
		template := &taskdomain.TaskTemplate{
			Title:       normalized.Title,
			Description: normalized.Description,
			Status:      normalized.Status,
			Recurrence:  *recurrence,
			CreatedAt:   now,
			UpdatedAt:   now,
		}

		if err := ensureOccurrencesInHorizon(template.Recurrence, now); err != nil {
			return nil, err
		}

		createdTemplate, createErr := s.repo.CreateTemplate(ctx, template)
		if createErr != nil {
			return nil, createErr
		}

		if deleteErr := s.repo.Delete(ctx, id); deleteErr != nil {
			return nil, deleteErr
		}

		generated, generateErr := s.generateForTemplate(ctx, createdTemplate, true)
		if generateErr != nil {
			return nil, generateErr
		}

		return &generated[0], nil
	}

	template, err := s.repo.GetTemplateByID(ctx, *existingTask.TemplateID)
	if err != nil {
		return nil, err
	}
	template.Title = normalized.Title
	template.Description = normalized.Description
	template.Status = normalized.Status
	if recurrence != nil {
		template.Recurrence = *recurrence
	}
	template.UpdatedAt = s.now().UTC()

	if err := ensureOccurrencesInHorizon(template.Recurrence, template.UpdatedAt); err != nil {
		return nil, err
	}

	updatedTemplate, err := s.repo.UpdateTemplate(ctx, template)
	if err != nil {
		return nil, err
	}

	if err := s.repo.DeleteGeneratedFromDate(ctx, updatedTemplate.ID, startOfDayUTC(s.now())); err != nil {
		return nil, err
	}

	generated, err := s.generateForTemplate(ctx, updatedTemplate, true)
	if err != nil {
		return nil, err
	}

	return &generated[0], nil
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	if id <= 0 {
		return fmt.Errorf("%w: id must be positive", ErrInvalidInput)
	}

	task, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if task.TemplateID != nil {
		return s.repo.DeleteTemplate(ctx, *task.TemplateID)
	}

	return s.repo.Delete(ctx, id)
}

func (s *Service) List(ctx context.Context) ([]taskdomain.Task, error) {
	templates, err := s.repo.ListTemplates(ctx)
	if err != nil {
		return nil, err
	}

	for i := range templates {
		if _, generateErr := s.generateForTemplate(ctx, &templates[i], false); generateErr != nil {
			return nil, generateErr
		}
	}

	return s.repo.List(ctx)
}

func validateCreateInput(input CreateInput) (CreateInput, *taskdomain.Recurrence, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.Description = strings.TrimSpace(input.Description)

	if input.Title == "" {
		return CreateInput{}, nil, fmt.Errorf("%w: title is required", ErrInvalidInput)
	}

	if input.Status == "" {
		input.Status = taskdomain.StatusNew
	}

	if !input.Status.Valid() {
		return CreateInput{}, nil, fmt.Errorf("%w: invalid status", ErrInvalidInput)
	}

	recurrence, err := validateRecurrenceInput(input.Recurrence)
	if err != nil {
		return CreateInput{}, nil, err
	}

	return input, recurrence, nil
}

func validateUpdateInput(input UpdateInput) (UpdateInput, *taskdomain.Recurrence, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.Description = strings.TrimSpace(input.Description)

	if input.Title == "" {
		return UpdateInput{}, nil, fmt.Errorf("%w: title is required", ErrInvalidInput)
	}

	if !input.Status.Valid() {
		return UpdateInput{}, nil, fmt.Errorf("%w: invalid status", ErrInvalidInput)
	}

	recurrence, err := validateRecurrenceInput(input.Recurrence)
	if err != nil {
		return UpdateInput{}, nil, err
	}

	return input, recurrence, nil
}

func validateRecurrenceInput(input *RecurrenceInput) (*taskdomain.Recurrence, error) {
	if input == nil {
		return nil, nil
	}

	if !input.Type.Valid() {
		return nil, fmt.Errorf("%w: invalid recurrence type", ErrInvalidInput)
	}

	recurrence := &taskdomain.Recurrence{Type: input.Type}
	switch input.Type {
	case taskdomain.RecurrenceDaily:
		if input.Every == nil || *input.Every <= 0 {
			return nil, fmt.Errorf("%w: recurrence.every must be positive for daily", ErrInvalidInput)
		}
		if input.Day != nil || len(input.SpecificDates) > 0 {
			return nil, fmt.Errorf("%w: daily recurrence does not support day or specific_dates", ErrInvalidInput)
		}

		startDate, endDate, err := parseStartEndDates(input.StartDate, input.EndDate)
		if err != nil {
			return nil, err
		}

		recurrence.EveryNDays = *input.Every
		recurrence.StartDate = startDate
		recurrence.EndDate = endDate
	case taskdomain.RecurrenceMonthly:
		if input.Day == nil || *input.Day < 1 || *input.Day > 30 {
			return nil, fmt.Errorf("%w: recurrence.day must be in range 1..30 for monthly", ErrInvalidInput)
		}
		if input.Every != nil || len(input.SpecificDates) > 0 {
			return nil, fmt.Errorf("%w: monthly recurrence does not support every or specific_dates", ErrInvalidInput)
		}

		startDate, endDate, err := parseStartEndDates(input.StartDate, input.EndDate)
		if err != nil {
			return nil, err
		}

		recurrence.MonthlyDay = *input.Day
		recurrence.StartDate = startDate
		recurrence.EndDate = endDate
	case taskdomain.RecurrenceSpecificDates:
		if input.Every != nil || input.Day != nil || strings.TrimSpace(input.StartDate) != "" || input.EndDate != nil {
			return nil, fmt.Errorf("%w: specific_dates recurrence supports only specific_dates field", ErrInvalidInput)
		}
		if len(input.SpecificDates) == 0 {
			return nil, fmt.Errorf("%w: specific_dates list cannot be empty", ErrInvalidInput)
		}

		parsedDates := make([]time.Time, 0, len(input.SpecificDates))
		seen := make(map[string]struct{}, len(input.SpecificDates))
		for _, rawDate := range input.SpecificDates {
			parsedDate, err := parseDate(rawDate)
			if err != nil {
				return nil, fmt.Errorf("%w: invalid recurrence.specific_dates value", ErrInvalidInput)
			}

			key := parsedDate.Format(recurrenceDateLayout)
			if _, exists := seen[key]; exists {
				return nil, fmt.Errorf("%w: duplicate date in recurrence.specific_dates", ErrInvalidInput)
			}
			seen[key] = struct{}{}
			parsedDates = append(parsedDates, parsedDate)
		}

		sort.Slice(parsedDates, func(i, j int) bool {
			return parsedDates[i].Before(parsedDates[j])
		})

		recurrence.SpecificDates = parsedDates
		recurrence.StartDate = parsedDates[0]
	case taskdomain.RecurrenceEvenDays, taskdomain.RecurrenceOddDays:
		if input.Every != nil || input.Day != nil || len(input.SpecificDates) > 0 {
			return nil, fmt.Errorf("%w: even_days/odd_days supports only start_date and end_date", ErrInvalidInput)
		}

		startDate, endDate, err := parseStartEndDates(input.StartDate, input.EndDate)
		if err != nil {
			return nil, err
		}

		recurrence.StartDate = startDate
		recurrence.EndDate = endDate
	default:
		return nil, fmt.Errorf("%w: invalid recurrence type", ErrInvalidInput)
	}

	return recurrence, nil
}

func parseStartEndDates(startDateRaw string, endDateRaw *string) (time.Time, *time.Time, error) {
	if strings.TrimSpace(startDateRaw) == "" {
		return time.Time{}, nil, fmt.Errorf("%w: recurrence.start_date is required", ErrInvalidInput)
	}

	startDate, err := parseDate(startDateRaw)
	if err != nil {
		return time.Time{}, nil, fmt.Errorf("%w: invalid recurrence.start_date", ErrInvalidInput)
	}

	if endDateRaw == nil {
		return startDate, nil, nil
	}

	endDate, err := parseDate(*endDateRaw)
	if err != nil {
		return time.Time{}, nil, fmt.Errorf("%w: invalid recurrence.end_date", ErrInvalidInput)
	}
	if endDate.Before(startDate) {
		return time.Time{}, nil, fmt.Errorf("%w: recurrence.end_date cannot be before recurrence.start_date", ErrInvalidInput)
	}

	return startDate, &endDate, nil
}

func parseDate(raw string) (time.Time, error) {
	parsed, err := time.Parse(recurrenceDateLayout, strings.TrimSpace(raw))
	if err != nil {
		return time.Time{}, err
	}

	return startOfDayUTC(parsed), nil
}

func (s *Service) generateForTemplate(ctx context.Context, template *taskdomain.TaskTemplate, failOnEmpty bool) ([]taskdomain.Task, error) {
	today := startOfDayUTC(s.now())
	horizonEnd := today.AddDate(0, 0, generationHorizon)
	dates := buildOccurrenceDates(template.Recurrence, today, horizonEnd)
	if len(dates) == 0 {
		if failOnEmpty {
			return nil, fmt.Errorf("%w: recurrence does not produce tasks in the next %d days", ErrInvalidInput, generationHorizon)
		}

		return nil, nil
	}

	now := s.now().UTC()
	generated := make([]taskdomain.Task, 0, len(dates))
	for _, date := range dates {
		templateID := template.ID
		scheduledFor := date
		toUpsert := &taskdomain.Task{
			Title:        template.Title,
			Description:  template.Description,
			Status:       template.Status,
			TemplateID:   &templateID,
			ScheduledFor: &scheduledFor,
			CreatedAt:    now,
			UpdatedAt:    now,
		}

		upserted, err := s.repo.UpsertGeneratedTask(ctx, toUpsert)
		if err != nil {
			return nil, err
		}

		generated = append(generated, *upserted)
	}

	sort.Slice(generated, func(i, j int) bool {
		if generated[i].ScheduledFor != nil && generated[j].ScheduledFor != nil {
			if generated[i].ScheduledFor.Equal(*generated[j].ScheduledFor) {
				return generated[i].ID < generated[j].ID
			}

			return generated[i].ScheduledFor.Before(*generated[j].ScheduledFor)
		}

		return generated[i].ID < generated[j].ID
	})

	return generated, nil
}

func buildOccurrenceDates(rule taskdomain.Recurrence, windowStart, windowEnd time.Time) []time.Time {
	windowStart = startOfDayUTC(windowStart)
	windowEnd = startOfDayUTC(windowEnd)

	if windowStart.Before(rule.StartDate) {
		windowStart = startOfDayUTC(rule.StartDate)
	}

	if rule.EndDate != nil {
		endDate := startOfDayUTC(*rule.EndDate)
		if endDate.Before(windowEnd) {
			windowEnd = endDate
		}
	}

	if windowStart.After(windowEnd) {
		return nil
	}

	specificDates := make(map[string]struct{}, len(rule.SpecificDates))
	for _, value := range rule.SpecificDates {
		specificDates[startOfDayUTC(value).Format(recurrenceDateLayout)] = struct{}{}
	}

	result := make([]time.Time, 0)
	for date := windowStart; !date.After(windowEnd); date = date.AddDate(0, 0, 1) {
		if matchesRecurrence(rule, date, specificDates) {
			result = append(result, date)
		}
	}

	return result
}

func matchesRecurrence(rule taskdomain.Recurrence, date time.Time, specificDates map[string]struct{}) bool {
	switch rule.Type {
	case taskdomain.RecurrenceDaily:
		diffDays := int(startOfDayUTC(date).Sub(startOfDayUTC(rule.StartDate)).Hours() / 24)
		return diffDays >= 0 && diffDays%rule.EveryNDays == 0
	case taskdomain.RecurrenceMonthly:
		return date.Day() == rule.MonthlyDay
	case taskdomain.RecurrenceSpecificDates:
		_, exists := specificDates[date.Format(recurrenceDateLayout)]
		return exists
	case taskdomain.RecurrenceEvenDays:
		return date.Day()%2 == 0
	case taskdomain.RecurrenceOddDays:
		return date.Day()%2 == 1
	default:
		return false
	}
}

func startOfDayUTC(value time.Time) time.Time {
	y, m, d := value.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func ensureOccurrencesInHorizon(rule taskdomain.Recurrence, now time.Time) error {
	today := startOfDayUTC(now)
	horizonEnd := today.AddDate(0, 0, generationHorizon)
	if len(buildOccurrenceDates(rule, today, horizonEnd)) == 0 {
		return fmt.Errorf("%w: recurrence does not produce tasks in the next %d days", ErrInvalidInput, generationHorizon)
	}

	return nil
}

package task

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	taskdomain "example.com/taskservice/internal/domain/task"
)

func TestValidateRecurrenceInput(t *testing.T) {
	t.Parallel()

	every2 := 2
	day31 := 31
	day15 := 15
	endDate := "2026-04-30"

	tests := []struct {
		name    string
		input   *RecurrenceInput
		wantErr bool
	}{
		{
			name: "valid_daily",
			input: &RecurrenceInput{
				Type:      taskdomain.RecurrenceDaily,
				Every:     &every2,
				StartDate: "2026-04-10",
				EndDate:   &endDate,
			},
		},
		{
			name: "daily_without_every",
			input: &RecurrenceInput{
				Type:      taskdomain.RecurrenceDaily,
				StartDate: "2026-04-10",
			},
			wantErr: true,
		},
		{
			name: "monthly_day_out_of_range",
			input: &RecurrenceInput{
				Type:      taskdomain.RecurrenceMonthly,
				Day:       &day31,
				StartDate: "2026-04-10",
			},
			wantErr: true,
		},
		{
			name: "specific_dates_with_start_date",
			input: &RecurrenceInput{
				Type:          taskdomain.RecurrenceSpecificDates,
				SpecificDates: []string{"2026-04-10"},
				StartDate:     "2026-04-01",
			},
			wantErr: true,
		},
		{
			name: "even_days_with_daily_field",
			input: &RecurrenceInput{
				Type:      taskdomain.RecurrenceEvenDays,
				Every:     &every2,
				StartDate: "2026-04-10",
			},
			wantErr: true,
		},
		{
			name: "valid_monthly",
			input: &RecurrenceInput{
				Type:      taskdomain.RecurrenceMonthly,
				Day:       &day15,
				StartDate: "2026-04-01",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := validateRecurrenceInput(tt.input)
			if tt.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}

func TestBuildOccurrenceDates(t *testing.T) {
	t.Parallel()

	start := mustParseDate(t, "2026-04-10")
	end := mustParseDate(t, "2026-04-20")
	windowStart := mustParseDate(t, "2026-04-10")
	windowEnd := mustParseDate(t, "2026-04-25")

	dailyRule := taskdomain.Recurrence{
		Type:       taskdomain.RecurrenceDaily,
		EveryNDays: 2,
		StartDate:  start,
		EndDate:    &end,
	}

	gotDaily := toDateStrings(buildOccurrenceDates(dailyRule, windowStart, windowEnd))
	wantDaily := []string{"2026-04-10", "2026-04-12", "2026-04-14", "2026-04-16", "2026-04-18", "2026-04-20"}
	assertEqualSlices(t, gotDaily, wantDaily)

	monthlyRule := taskdomain.Recurrence{
		Type:       taskdomain.RecurrenceMonthly,
		MonthlyDay: 15,
		StartDate:  mustParseDate(t, "2026-04-01"),
		EndDate:    datePtr(mustParseDate(t, "2026-05-30")),
	}

	gotMonthly := toDateStrings(buildOccurrenceDates(monthlyRule, mustParseDate(t, "2026-04-10"), mustParseDate(t, "2026-05-20")))
	wantMonthly := []string{"2026-04-15", "2026-05-15"}
	assertEqualSlices(t, gotMonthly, wantMonthly)

	specificRule := taskdomain.Recurrence{
		Type:          taskdomain.RecurrenceSpecificDates,
		SpecificDates: []time.Time{mustParseDate(t, "2026-04-11"), mustParseDate(t, "2026-04-13"), mustParseDate(t, "2026-04-20")},
		StartDate:     mustParseDate(t, "2026-04-11"),
	}

	gotSpecific := toDateStrings(buildOccurrenceDates(specificRule, mustParseDate(t, "2026-04-10"), mustParseDate(t, "2026-04-18")))
	wantSpecific := []string{"2026-04-11", "2026-04-13"}
	assertEqualSlices(t, gotSpecific, wantSpecific)
}

func TestServiceRecurringCreateAndNoDuplicates(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	svc := NewService(repo)
	svc.now = func() time.Time {
		return time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	}

	ctx := context.Background()
	endDate := "2026-04-12"
	recurrenceEvery := 1

	created, err := svc.Create(ctx, CreateInput{
		Title:       "Daily task",
		Description: "Check logs",
		Status:      taskdomain.StatusNew,
		Recurrence: &RecurrenceInput{
			Type:      taskdomain.RecurrenceDaily,
			Every:     &recurrenceEvery,
			StartDate: "2026-04-10",
			EndDate:   &endDate,
		},
	})
	if err != nil {
		t.Fatalf("create recurring task failed: %v", err)
	}

	if created.TemplateID == nil || created.ScheduledFor == nil {
		t.Fatalf("expected generated task with template reference and schedule")
	}

	if _, err := svc.List(ctx); err != nil {
		t.Fatalf("first list failed: %v", err)
	}
	if _, err := svc.List(ctx); err != nil {
		t.Fatalf("second list failed: %v", err)
	}

	tasks, err := svc.List(ctx)
	if err != nil {
		t.Fatalf("third list failed: %v", err)
	}

	if len(tasks) != 3 {
		t.Fatalf("expected 3 generated tasks, got %d", len(tasks))
	}

	seen := make(map[string]struct{}, len(tasks))
	for _, task := range tasks {
		key := recurrenceKey(task.TemplateID, task.ScheduledFor)
		if _, exists := seen[key]; exists {
			t.Fatalf("duplicate generated task for key %s", key)
		}
		seen[key] = struct{}{}
	}
}

func TestServiceUpdateRecurringTask(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	svc := NewService(repo)
	svc.now = func() time.Time {
		return time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	}

	ctx := context.Background()
	startDate := "2026-04-10"
	endDate := "2026-04-20"
	created, err := svc.Create(ctx, CreateInput{
		Title:       "Parity task",
		Description: "Original",
		Status:      taskdomain.StatusNew,
		Recurrence: &RecurrenceInput{
			Type:      taskdomain.RecurrenceOddDays,
			StartDate: startDate,
			EndDate:   &endDate,
		},
	})
	if err != nil {
		t.Fatalf("create recurring task failed: %v", err)
	}

	monthlyDay := 12
	updateEnd := "2026-04-30"
	updated, err := svc.Update(ctx, created.ID, UpdateInput{
		Title:       "Monthly task",
		Description: "Updated",
		Status:      taskdomain.StatusInProgress,
		Recurrence: &RecurrenceInput{
			Type:      taskdomain.RecurrenceMonthly,
			Day:       &monthlyDay,
			StartDate: "2026-04-10",
			EndDate:   &updateEnd,
		},
	})
	if err != nil {
		t.Fatalf("update recurring task failed: %v", err)
	}

	if updated.ScheduledFor == nil {
		t.Fatalf("updated recurring task must have scheduled date")
	}

	gotDate := updated.ScheduledFor.Format("2006-01-02")
	if gotDate != "2026-04-12" {
		t.Fatalf("expected updated schedule 2026-04-12, got %s", gotDate)
	}

	tasks, err := svc.List(ctx)
	if err != nil {
		t.Fatalf("list tasks failed: %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("expected 1 generated task after update, got %d", len(tasks))
	}

	if tasks[0].ScheduledFor == nil || tasks[0].ScheduledFor.Format("2006-01-02") != "2026-04-12" {
		t.Fatalf("expected only monthly generated date 2026-04-12")
	}
}

type fakeRepository struct {
	nextTaskID     int64
	nextTemplateID int64
	tasks          map[int64]taskdomain.Task
	templates      map[int64]taskdomain.TaskTemplate
	byTemplateDate map[string]int64
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{
		nextTaskID:     1,
		nextTemplateID: 1,
		tasks:          make(map[int64]taskdomain.Task),
		templates:      make(map[int64]taskdomain.TaskTemplate),
		byTemplateDate: make(map[string]int64),
	}
}

func (f *fakeRepository) Create(_ context.Context, task *taskdomain.Task) (*taskdomain.Task, error) {
	taskCopy := cloneTask(task)
	taskCopy.ID = f.nextTaskID
	f.nextTaskID++
	f.tasks[taskCopy.ID] = taskCopy
	return cloneTaskPtr(&taskCopy), nil
}

func (f *fakeRepository) GetByID(_ context.Context, id int64) (*taskdomain.Task, error) {
	task, ok := f.tasks[id]
	if !ok {
		return nil, taskdomain.ErrNotFound
	}

	return cloneTaskPtr(&task), nil
}

func (f *fakeRepository) Update(_ context.Context, task *taskdomain.Task) (*taskdomain.Task, error) {
	current, ok := f.tasks[task.ID]
	if !ok {
		return nil, taskdomain.ErrNotFound
	}

	current.Title = task.Title
	current.Description = task.Description
	current.Status = task.Status
	current.UpdatedAt = task.UpdatedAt
	f.tasks[task.ID] = current
	return cloneTaskPtr(&current), nil
}

func (f *fakeRepository) Delete(_ context.Context, id int64) error {
	if _, ok := f.tasks[id]; !ok {
		return taskdomain.ErrNotFound
	}

	delete(f.tasks, id)

	for key, taskID := range f.byTemplateDate {
		if taskID == id {
			delete(f.byTemplateDate, key)
		}
	}

	return nil
}

func (f *fakeRepository) List(_ context.Context) ([]taskdomain.Task, error) {
	result := make([]taskdomain.Task, 0, len(f.tasks))
	for _, task := range f.tasks {
		result = append(result, cloneTask(&task))
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].ID > result[j].ID
	})

	return result, nil
}

func (f *fakeRepository) CreateTemplate(_ context.Context, template *taskdomain.TaskTemplate) (*taskdomain.TaskTemplate, error) {
	templateCopy := cloneTemplate(template)
	templateCopy.ID = f.nextTemplateID
	f.nextTemplateID++
	f.templates[templateCopy.ID] = templateCopy
	return cloneTemplatePtr(&templateCopy), nil
}

func (f *fakeRepository) GetTemplateByID(_ context.Context, id int64) (*taskdomain.TaskTemplate, error) {
	template, ok := f.templates[id]
	if !ok {
		return nil, taskdomain.ErrNotFound
	}

	return cloneTemplatePtr(&template), nil
}

func (f *fakeRepository) UpdateTemplate(_ context.Context, template *taskdomain.TaskTemplate) (*taskdomain.TaskTemplate, error) {
	if _, ok := f.templates[template.ID]; !ok {
		return nil, taskdomain.ErrNotFound
	}

	templateCopy := cloneTemplate(template)
	f.templates[template.ID] = templateCopy
	return cloneTemplatePtr(&templateCopy), nil
}

func (f *fakeRepository) DeleteTemplate(_ context.Context, id int64) error {
	if _, ok := f.templates[id]; !ok {
		return taskdomain.ErrNotFound
	}

	delete(f.templates, id)
	for taskID, task := range f.tasks {
		if task.TemplateID != nil && *task.TemplateID == id {
			delete(f.tasks, taskID)
		}
	}

	for key := range f.byTemplateDate {
		if len(key) > 0 && keyPrefixMatchesTemplate(key, id) {
			delete(f.byTemplateDate, key)
		}
	}

	return nil
}

func (f *fakeRepository) ListTemplates(_ context.Context) ([]taskdomain.TaskTemplate, error) {
	result := make([]taskdomain.TaskTemplate, 0, len(f.templates))
	for _, template := range f.templates {
		result = append(result, cloneTemplate(&template))
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})

	return result, nil
}

func (f *fakeRepository) UpsertGeneratedTask(_ context.Context, task *taskdomain.Task) (*taskdomain.Task, error) {
	key := recurrenceKey(task.TemplateID, task.ScheduledFor)
	if existingID, ok := f.byTemplateDate[key]; ok {
		current := f.tasks[existingID]
		current.Title = task.Title
		current.Description = task.Description
		current.Status = task.Status
		current.UpdatedAt = task.UpdatedAt
		f.tasks[existingID] = current
		return cloneTaskPtr(&current), nil
	}

	taskCopy := cloneTask(task)
	taskCopy.ID = f.nextTaskID
	f.nextTaskID++
	f.tasks[taskCopy.ID] = taskCopy
	f.byTemplateDate[key] = taskCopy.ID
	return cloneTaskPtr(&taskCopy), nil
}

func (f *fakeRepository) DeleteGeneratedFromDate(_ context.Context, templateID int64, fromDate time.Time) error {
	fromDate = startOfDayUTC(fromDate)
	for taskID, task := range f.tasks {
		if task.TemplateID == nil || *task.TemplateID != templateID || task.ScheduledFor == nil {
			continue
		}

		if !task.ScheduledFor.Before(fromDate) {
			delete(f.tasks, taskID)
			delete(f.byTemplateDate, recurrenceKey(task.TemplateID, task.ScheduledFor))
		}
	}

	return nil
}

func cloneTask(task *taskdomain.Task) taskdomain.Task {
	cloned := *task
	if task.TemplateID != nil {
		templateID := *task.TemplateID
		cloned.TemplateID = &templateID
	}
	if task.ScheduledFor != nil {
		scheduledFor := *task.ScheduledFor
		cloned.ScheduledFor = &scheduledFor
	}
	return cloned
}

func cloneTaskPtr(task *taskdomain.Task) *taskdomain.Task {
	cloned := cloneTask(task)
	return &cloned
}

func cloneTemplate(template *taskdomain.TaskTemplate) taskdomain.TaskTemplate {
	cloned := *template
	cloned.Recurrence = template.Recurrence
	if template.Recurrence.SpecificDates != nil {
		cloned.Recurrence.SpecificDates = append([]time.Time(nil), template.Recurrence.SpecificDates...)
	}
	if template.Recurrence.EndDate != nil {
		endDate := *template.Recurrence.EndDate
		cloned.Recurrence.EndDate = &endDate
	}
	return cloned
}

func cloneTemplatePtr(template *taskdomain.TaskTemplate) *taskdomain.TaskTemplate {
	cloned := cloneTemplate(template)
	return &cloned
}

func recurrenceKey(templateID *int64, scheduledFor *time.Time) string {
	if templateID == nil || scheduledFor == nil {
		return ""
	}

	return formatTemplateDateKey(*templateID, *scheduledFor)
}

func formatTemplateDateKey(templateID int64, date time.Time) string {
	return date.Format("2006-01-02") + ":" + strconv.FormatInt(templateID, 10)
}

func keyPrefixMatchesTemplate(key string, templateID int64) bool {
	return strings.HasSuffix(key, ":"+strconv.FormatInt(templateID, 10))
}

func mustParseDate(t *testing.T, raw string) time.Time {
	t.Helper()
	value, err := time.Parse("2006-01-02", raw)
	if err != nil {
		t.Fatalf("parse date %s: %v", raw, err)
	}
	return startOfDayUTC(value)
}

func toDateStrings(dates []time.Time) []string {
	result := make([]string, 0, len(dates))
	for _, date := range dates {
		result = append(result, date.Format("2006-01-02"))
	}
	return result
}

func assertEqualSlices(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("length mismatch: got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("mismatch at index %d: got %s, want %s", i, got[i], want[i])
		}
	}
}

func datePtr(value time.Time) *time.Time {
	return &value
}

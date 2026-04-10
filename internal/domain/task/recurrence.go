package task

import "time"

type RecurrenceType string

const (
	RecurrenceDaily         RecurrenceType = "daily"
	RecurrenceMonthly       RecurrenceType = "monthly"
	RecurrenceSpecificDates RecurrenceType = "specific_dates"
	RecurrenceEvenDays      RecurrenceType = "even_days"
	RecurrenceOddDays       RecurrenceType = "odd_days"
)

func (t RecurrenceType) Valid() bool {
	switch t {
	case RecurrenceDaily, RecurrenceMonthly, RecurrenceSpecificDates, RecurrenceEvenDays, RecurrenceOddDays:
		return true
	default:
		return false
	}
}

type Recurrence struct {
	Type          RecurrenceType
	EveryNDays    int
	MonthlyDay    int
	SpecificDates []time.Time
	StartDate     time.Time
	EndDate       *time.Time
}

type TaskTemplate struct {
	ID          int64
	Title       string
	Description string
	Status      Status
	Recurrence  Recurrence
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

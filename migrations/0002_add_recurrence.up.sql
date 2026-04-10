CREATE TABLE IF NOT EXISTS task_templates (
	id BIGSERIAL PRIMARY KEY,
	title TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL,
	recurrence_type TEXT NOT NULL,
	every_n_days INTEGER NULL,
	monthly_day INTEGER NULL,
	specific_dates JSONB NOT NULL DEFAULT '[]'::jsonb,
	start_date DATE NOT NULL,
	end_date DATE NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	CONSTRAINT chk_task_templates_recurrence_type CHECK (
		recurrence_type IN ('daily', 'monthly', 'specific_dates', 'even_days', 'odd_days')
	)
);

ALTER TABLE tasks
	ADD COLUMN IF NOT EXISTS template_id BIGINT NULL REFERENCES task_templates(id) ON DELETE CASCADE,
	ADD COLUMN IF NOT EXISTS scheduled_for DATE NULL;

DO $$
BEGIN
	IF NOT EXISTS (
		SELECT 1
		FROM pg_constraint
		WHERE conname = 'chk_tasks_template_schedule_pair'
	) THEN
		ALTER TABLE tasks
			ADD CONSTRAINT chk_tasks_template_schedule_pair
				CHECK (
					(template_id IS NULL AND scheduled_for IS NULL)
					OR
					(template_id IS NOT NULL AND scheduled_for IS NOT NULL)
				);
	END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS ux_tasks_template_scheduled_for ON tasks (template_id, scheduled_for);
CREATE INDEX IF NOT EXISTS idx_tasks_template_id ON tasks (template_id);
CREATE INDEX IF NOT EXISTS idx_tasks_scheduled_for ON tasks (scheduled_for);

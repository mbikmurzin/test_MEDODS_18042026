# Task Service

Task tracker HTTP API on Go with PostgreSQL.  
Now supports recurring task templates and deterministic generation of task instances.

## Requirements

- Go `1.23+`
- Docker and Docker Compose

## Run

```bash
docker compose up --build
```

Service will be available at `http://localhost:8080`.

If you previously started PostgreSQL with old schema, recreate the volume:

```bash
docker compose down -v
docker compose up --build
```

`docker-compose.yml` mounts the full `migrations/` directory into Postgres init folder.

## Swagger

- Swagger UI: `http://localhost:8080/swagger/`
- OpenAPI JSON: `http://localhost:8080/swagger/openapi.json`

## API Base Path

`/api/v1`

Main routes:

- `POST /tasks`
- `GET /tasks`
- `GET /tasks/{id}`
- `PUT /tasks/{id}`
- `DELETE /tasks/{id}`

## Recurrence Design

### Data Model

Implemented Option A:

- `task_templates` table stores recurring template + recurrence rule.
- `tasks` table stores generated task instances and regular one-off tasks.
- `tasks.template_id` references `task_templates.id` (nullable).
- `tasks.scheduled_for` stores generated date (nullable).
- Unique index on `(template_id, scheduled_for)` guarantees no duplicate generated instances.

### Supported Recurrence Types

- `daily` with `every` (every N days)
- `monthly` with `day` (1..30)
- `specific_dates` with explicit date list
- `even_days` (day-of-month parity)
- `odd_days` (day-of-month parity)

### Generation Rules

- Generation horizon is fixed at 30 days ahead from current UTC date.
- Generation is deterministic and idempotent.
- Duplicates are prevented by unique index + upsert.
- Generation is triggered on:
1. `POST /tasks` for recurring payloads.
2. `PUT /tasks/{id}` when updating recurring task/template.
3. `GET /tasks` for active templates (refresh window).

### Recurring Template vs Generated Task

- Template fields (title, description, status, recurrence rule) are stored in `task_templates`.
- Generated task instances are normal rows in `tasks`.
- Generated rows include:
1. `template_id`
2. `scheduled_for`

### Update/Delete Behavior

- Updating a recurring task updates its template and regenerates future tasks from today.
- Deleting a recurring task deletes the whole template and all generated tasks via FK cascade.
- Non-recurring CRUD behavior remains unchanged.

## Recurrence Payload

`POST /api/v1/tasks` and `PUT /api/v1/tasks/{id}` accept:

```json
{
  "title": "Pay utilities",
  "description": "Monthly payment",
  "status": "new",
  "recurrence": {
    "type": "daily",
    "every": 2,
    "start_date": "2026-04-10",
    "end_date": "2026-05-10"
  }
}
```

## Validation Rules

Strict validation is applied in usecase layer:

- `daily`: requires `every > 0`, requires `start_date`.
- `monthly`: requires `day` in `1..30`, requires `start_date`.
- `specific_dates`: requires non-empty unique `specific_dates`; other recurrence fields are rejected.
- `even_days`/`odd_days`: require `start_date`; `every`, `day`, `specific_dates` are rejected.
- Invalid date format is rejected (`YYYY-MM-DD` expected).
- `end_date` cannot be before `start_date`.
- Unknown JSON fields are rejected by HTTP decoder.

All recurrence and scheduling calculations use UTC date boundaries.

## Manual Testing

### 1) Create regular task

```bash
curl -X POST http://localhost:8080/api/v1/tasks \
  -H "Content-Type: application/json" \
  -d '{"title":"One-off task","description":"Manual","status":"new"}'
```

### 2) Create recurring daily task

```bash
curl -X POST http://localhost:8080/api/v1/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "title":"Daily check",
    "description":"Routine",
    "status":"new",
    "recurrence":{
      "type":"daily",
      "every":1,
      "start_date":"2026-04-10",
      "end_date":"2026-04-20"
    }
  }'
```

### 3) List tasks (also refreshes future generation)

```bash
curl http://localhost:8080/api/v1/tasks
```

### 4) Update recurring task recurrence

Use an id from recurring generated result:

```bash
curl -X PUT http://localhost:8080/api/v1/tasks/1 \
  -H "Content-Type: application/json" \
  -d '{
    "title":"Updated recurring",
    "description":"Changed rule",
    "status":"in_progress",
    "recurrence":{
      "type":"monthly",
      "day":12,
      "start_date":"2026-04-10",
      "end_date":"2026-06-10"
    }
  }'
```

### 5) Delete recurring template through task id

```bash
curl -X DELETE http://localhost:8080/api/v1/tasks/1
```

## Tests

Added tests in `internal/usecase/task/service_recurrence_test.go`:

- recurrence validation
- deterministic generation logic
- no duplicates / idempotency
- recurring create and recurring update flows

Run:

```bash
go test ./...
```

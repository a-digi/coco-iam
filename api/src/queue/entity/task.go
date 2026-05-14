package entity

// QueueTask is the ORM entity used to expose the `queue_tasks` table via the
// admin read-only API. It mirrors the DB schema. The queue system itself
// writes this table via direct SQL; this entity is for GET responses only.
//
// Payload is not stored in the DB — the bytes live on disk under
// `./data/queue-messages/YYYY/MM/DD/<id>.json` and are served via the
// dedicated `GET /api/v1/admin/queue_tasks/{id}/payload` endpoint.
type QueueTask struct {
	_             struct{} `table:"queue_tasks"`
	ID            string   `db:"id" dbtype:"UUID" nullable:"false" json:"id"`
	QueueName     string   `db:"queue_name" dbtype:"TEXT" nullable:"false" json:"queue_name"`
	Status        string   `db:"status" dbtype:"TEXT" nullable:"false" default:"pending" json:"status"`
	Attempts      int      `db:"attempts" dbtype:"INTEGER" nullable:"false" default:"0" json:"attempts"`
	MaxAttempts   int      `db:"max_attempts" dbtype:"INTEGER" nullable:"false" default:"3" json:"max_attempts"`
	LastError     string   `db:"last_error" dbtype:"TEXT" nullable:"false" default:"" json:"last_error"`
	NextAttemptAt string   `db:"next_attempt_at" dbtype:"DATETIME" nullable:"true" json:"next_attempt_at"`
	CreatedAt     string   `db:"created_at" dbtype:"DATETIME" nullable:"true" json:"created_at"`
	UpdatedAt     string   `db:"updated_at" dbtype:"DATETIME" nullable:"true" json:"updated_at"`
	CompletedAt   string   `db:"completed_at" dbtype:"DATETIME" nullable:"true" json:"completed_at"`
}

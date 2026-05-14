package entity

type User struct {
	_              struct{} `table:"users"`
	ID             string   `db:"id" dbtype:"UUID" nullable:"false" json:"id"`
	Username       string   `db:"username" dbtype:"TEXT" nullable:"false" json:"username"`
	Email          string   `db:"email" dbtype:"TEXT" nullable:"false" json:"email"`
	OrganizationID string   `db:"organization_id" dbtype:"TEXT" nullable:"false" json:"organization_id"`
	CreatedAt      string   `db:"created_at" dbtype:"DATETIME" nullable:"true" json:"created_at"`
	IsActive       bool     `db:"is_active" dbtype:"BOOLEAN" nullable:"false" default:"true" json:"is_active"`
}

package entity

type OrganizationGroup struct {
	_                struct{} `table:"user_groups"`
	ID               string   `db:"id" dbtype:"UUID" nullable:"false" json:"id"`
	Title            string   `db:"title" dbtype:"TEXT" nullable:"false" json:"title"`
	GroupDescription string   `db:"group_description" dbtype:"TEXT" nullable:"false" default:"" json:"group_description"`
	OrganizationID   string   `db:"organization_id" dbtype:"TEXT" nullable:"false" default:"" json:"organization_id"`
	CreatedAt        string   `db:"created_at" dbtype:"DATETIME" nullable:"true" json:"created_at"`
	IsActive         bool     `db:"is_active" dbtype:"BOOLEAN" nullable:"false" default:"true" json:"is_active"`
}

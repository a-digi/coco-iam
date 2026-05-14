package entity

type AdminPassword struct {
	_         struct{} `table:"admin_auth_password"`
	UserId    string   `db:"user_id" dbtype:"UUID" nullable:"false" json:"user_id"`
	Password  string   `db:"password" dbtype:"TEXT" nullable:"false" json:"password"`
	CreatedAt string   `db:"created_at" dbtype:"DATETIME" nullable:"true" json:"created_at"`
	IsActive  bool     `db:"is_active" dbtype:"BOOLEAN" nullable:"false" default:"true" json:"is_active"`
}

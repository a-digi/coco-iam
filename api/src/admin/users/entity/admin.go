package entity

type User struct {
	_            struct{} `table:"admin_users"`
	ID           string   `db:"id" dbtype:"UUID" nullable:"true" json:"id"`
	Username     string   `db:"username" dbtype:"TEXT" nullable:"false" json:"username"`
	Email        string   `db:"email" dbtype:"TEXT" nullable:"false" json:"email"`
	CreatedAt    string   `db:"created_at" dbtype:"DATETIME" nullable:"true" json:"created_at"`
	IsSuperAdmin bool     `db:"is_super_admin" dbtype:"BOOLEAN" nullable:"false" default:"false" json:"is_super_admin"`
	IsActive     bool     `db:"is_active" dbtype:"BOOLEAN" nullable:"false" default:"true" json:"is_active"`
}

type UserLogin struct {
	_            struct{} `table:"admin_users"`
	ID           string   `db:"id" dbtype:"UUID" nullable:"true" json:"id"`
	Username     string   `db:"username" dbtype:"TEXT" nullable:"false" json:"username"`
	Email        string   `db:"email" dbtype:"TEXT" nullable:"false" json:"email"`
	IsSuperAdmin bool     `db:"is_super_admin" dbtype:"BOOLEAN" nullable:"false" default:"false" json:"is_super_admin"`
}

func (u UserLogin) GetUsername() string { return u.Username }
func (u UserLogin) GetPassword() string { return "" }

type Count struct {
	_     struct{} `table:"admin_users"`
	Total string   `db:"total" dbtype:"TEXT" nullable:"false" json:"total"`
}

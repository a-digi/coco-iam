package entity

// Standard represents a row in the 'standard' table.
type Standard struct {
	_         struct{} `table:"standard"`
	ID        string   `db:"id" dbtype:"UUID" nullable:"true" json:"id"`
	Title     string   `db:"title" dbtype:"TEXT" nullable:"false" json:"title"`
	Version   string   `db:"version" dbtype:"TEXT" nullable:"false" json:"version"`
	FilePath  string   `db:"file_path" dbtype:"TEXT" nullable:"false" json:"file_path"`
	FileHash  string   `db:"file_hash" dbtype:"TEXT" nullable:"false" json:"file_hash"`
	CreatedAt string   `db:"created_at" dbtype:"DATETIME" nullable:"true" json:"created_at"`
	IsActive  bool     `db:"is_active" dbtype:"BOOLEAN" nullable:"false" default:"true" json:"is_active"`
}

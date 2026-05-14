package entity

// ApplicationScope represents a permission identifier exposed by an
// application. The `scope_id` is a colon-separated identifier matching
// `^[a-zA-Z_]+(:[a-zA-Z_]+)*$` — the same format as admin scopes. Two apps
// can each define a scope with the same scope_id (scoped per application).
type ApplicationScope struct {
	_             struct{} `table:"application_scopes"`
	ID            string   `db:"id" dbtype:"UUID" nullable:"false" json:"id"`
	ApplicationID string   `db:"application_id" dbtype:"TEXT" nullable:"false" json:"application_id"`
	ScopeID       string   `db:"scope_id" dbtype:"TEXT" nullable:"false" json:"scope_id"`
	Description   string   `db:"description" dbtype:"TEXT" nullable:"false" default:"" json:"description"`
	// ResourceIDs is a JSON array of opaque id strings. Any user that
	// holds this scope inherits the constraint — the public API only
	// allows the scope to be applied against these ids. Empty array
	// means unconstrained (the scope applies to everything).
	ResourceIDs string `db:"resource_ids" dbtype:"TEXT" nullable:"false" default:"[]" json:"resource_ids"`
	CreatedAt   string `db:"created_at" dbtype:"DATETIME" nullable:"true" json:"created_at"`
	IsActive    bool   `db:"is_active" dbtype:"BOOLEAN" nullable:"false" default:"true" json:"is_active"`
}

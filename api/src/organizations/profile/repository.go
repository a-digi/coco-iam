package profile

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/a-digi/coco-iam/src/organizations/profile/entity"
	"github.com/a-digi/coco-orm/orm"
	"github.com/google/uuid"
)

// Repository wraps a per-org DatabaseManager and provides CRUD helpers
// for profile fields and user profiles. All methods operate on whatever
// DB the caller passed via OrgDBRegistry.
type Repository struct {
	DB *orm.DatabaseManager
}

func NewRepository(db *orm.DatabaseManager) *Repository {
	return &Repository{DB: db}
}

// --- profile_fields ---

// ListFields returns all fields, ordered by order_index then created_at.
// When activeOnly is true, is_active = 0 rows are filtered out.
func (r *Repository) ListFields(activeOnly bool) ([]entity.ProfileField, error) {
	q := `SELECT id, name, label, description, data_type, is_required,
	             min_value, max_value, options_json, regex,
	             accept_mime, max_bytes, order_index,
	             is_active, created_at, updated_at
	      FROM profile_fields`
	if activeOnly {
		q += ` WHERE is_active = 1`
	}
	q += ` ORDER BY order_index ASC, created_at ASC`

	rows, err := r.DB.Connector.DB.Query(q)
	if err != nil {
		return nil, fmt.Errorf("list fields: %w", err)
	}
	defer rows.Close()

	out := []entity.ProfileField{}
	for rows.Next() {
		var f entity.ProfileField
		var optionsJSON string
		var minVal, maxVal sql.NullInt64
		if err := rows.Scan(
			&f.ID, &f.Name, &f.Label, &f.Description, &f.DataType, &f.IsRequired,
			&minVal, &maxVal, &optionsJSON, &f.Regex,
			&f.AcceptMime, &f.MaxBytes, &f.OrderIndex,
			&f.IsActive, &f.CreatedAt, &f.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan field: %w", err)
		}
		if minVal.Valid {
			v := int(minVal.Int64)
			f.MinValue = &v
		}
		if maxVal.Valid {
			v := int(maxVal.Int64)
			f.MaxValue = &v
		}
		if optionsJSON != "" {
			var opts []string
			_ = json.Unmarshal([]byte(optionsJSON), &opts)
			f.Options = opts
		} else {
			f.Options = []string{}
		}
		out = append(out, f)
	}
	return out, nil
}

// GetField fetches a single field by id.
func (r *Repository) GetField(id string) (*entity.ProfileField, error) {
	rows, err := r.DB.Connector.DB.Query(
		`SELECT id, name, label, description, data_type, is_required,
		        min_value, max_value, options_json, regex,
		        accept_mime, max_bytes, order_index,
		        is_active, created_at, updated_at
		 FROM profile_fields WHERE id = ? LIMIT 1`,
		id,
	)
	if err != nil {
		return nil, fmt.Errorf("get field: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, nil
	}
	var f entity.ProfileField
	var optionsJSON string
	var minVal, maxVal sql.NullInt64
	if err := rows.Scan(
		&f.ID, &f.Name, &f.Label, &f.Description, &f.DataType, &f.IsRequired,
		&minVal, &maxVal, &optionsJSON, &f.Regex,
		&f.AcceptMime, &f.MaxBytes, &f.OrderIndex,
		&f.IsActive, &f.CreatedAt, &f.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if minVal.Valid {
		v := int(minVal.Int64)
		f.MinValue = &v
	}
	if maxVal.Valid {
		v := int(maxVal.Int64)
		f.MaxValue = &v
	}
	if optionsJSON != "" {
		var opts []string
		_ = json.Unmarshal([]byte(optionsJSON), &opts)
		f.Options = opts
	} else {
		f.Options = []string{}
	}
	return &f, nil
}

// CreateField inserts a new field. The caller must have validated the shape.
// order_index defaults to (max + 1) when the caller passes OrderIndex == 0.
func (r *Repository) CreateField(f *entity.ProfileField) error {
	if f.ID == "" {
		f.ID = uuid.New().String()
	}
	if f.Options == nil {
		f.Options = []string{}
	}

	if f.OrderIndex == 0 {
		row := r.DB.Connector.DB.QueryRow(`SELECT COALESCE(MAX(order_index), 0) + 1 FROM profile_fields`)
		if err := row.Scan(&f.OrderIndex); err != nil {
			return fmt.Errorf("compute order: %w", err)
		}
	}

	opts, err := json.Marshal(f.Options)
	if err != nil {
		return fmt.Errorf("marshal options: %w", err)
	}

	_, err = r.DB.Connector.DB.Exec(
		`INSERT INTO profile_fields
			(id, name, label, description, data_type, is_required,
			 min_value, max_value, options_json, regex,
			 accept_mime, max_bytes, order_index, is_active,
			 created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		f.ID, f.Name, f.Label, f.Description, f.DataType, f.IsRequired,
		nullableInt(f.MinValue), nullableInt(f.MaxValue), string(opts), f.Regex,
		f.AcceptMime, f.MaxBytes, f.OrderIndex, true,
		time.Now().UTC().Format(time.RFC3339), time.Now().UTC().Format(time.RFC3339),
	)
	return err
}

// UpdateField applies the passed field's fields (excluding order/id/is_active) to the row.
func (r *Repository) UpdateField(f *entity.ProfileField) error {
	if f.Options == nil {
		f.Options = []string{}
	}
	opts, err := json.Marshal(f.Options)
	if err != nil {
		return fmt.Errorf("marshal options: %w", err)
	}
	_, err = r.DB.Connector.DB.Exec(
		`UPDATE profile_fields SET
			label = ?, description = ?, data_type = ?, is_required = ?,
			min_value = ?, max_value = ?, options_json = ?, regex = ?,
			accept_mime = ?, max_bytes = ?,
			updated_at = ?
		 WHERE id = ?`,
		f.Label, f.Description, f.DataType, f.IsRequired,
		nullableInt(f.MinValue), nullableInt(f.MaxValue), string(opts), f.Regex,
		f.AcceptMime, f.MaxBytes,
		time.Now().UTC().Format(time.RFC3339),
		f.ID,
	)
	return err
}

// SoftDeleteField marks a field is_active = 0.
func (r *Repository) SoftDeleteField(id string) error {
	_, err := r.DB.Connector.DB.Exec(
		`UPDATE profile_fields SET is_active = 0, updated_at = ? WHERE id = ?`,
		time.Now().UTC().Format(time.RFC3339), id,
	)
	return err
}

// ReorderFields sets order_index = position of id in the passed slice.
func (r *Repository) ReorderFields(orderedIDs []string) error {
	tx, err := r.DB.Connector.DB.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.Prepare(`UPDATE profile_fields SET order_index = ?, updated_at = ? WHERE id = ?`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now().UTC().Format(time.RFC3339)
	for idx, id := range orderedIDs {
		if _, err := stmt.Exec(idx+1, now, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// --- user_profiles ---

// GetUserProfile returns the profile row or nil if missing.
func (r *Repository) GetUserProfile(userID string) (*entity.UserProfile, error) {
	rows, err := r.DB.Connector.DB.Query(
		`SELECT user_id, profile_data, updated_at FROM user_profiles WHERE user_id = ? LIMIT 1`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("get user profile: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, nil
	}
	var p entity.UserProfile
	var dataJSON string
	if err := rows.Scan(&p.UserID, &dataJSON, &p.UpdatedAt); err != nil {
		return nil, err
	}
	data := map[string]interface{}{}
	if dataJSON != "" {
		_ = json.Unmarshal([]byte(dataJSON), &data)
	}
	p.ProfileData = data
	return &p, nil
}

// UpsertUserProfile creates or replaces a user's profile data.
func (r *Repository) UpsertUserProfile(userID string, data map[string]interface{}) error {
	blob, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal profile data: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = r.DB.Connector.DB.Exec(
		`INSERT INTO user_profiles (user_id, profile_data, updated_at)
		 VALUES (?, ?, ?)
		 ON CONFLICT(user_id) DO UPDATE SET profile_data = excluded.profile_data, updated_at = excluded.updated_at`,
		userID, string(blob), now,
	)
	return err
}

func nullableInt(p *int) interface{} {
	if p == nil {
		return nil
	}
	return *p
}

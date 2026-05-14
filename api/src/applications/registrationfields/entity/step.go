// Package entity defines the on-disk row shapes for application
// registration schema: one row per step in
// application_registration_steps, one row per field in
// application_registration_fields. Both live in the per-org
// profiles.db alongside profile_fields.
package entity

import "time"

// Step is one stage of a registration wizard. A single-step
// registration is simply an application with exactly one step row.
type Step struct {
	ID            string    `json:"id"`
	ApplicationID string    `json:"application_id"`
	OrderIndex    int       `json:"order_index"`
	Title         string    `json:"title"`
	Description   string    `json:"description"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

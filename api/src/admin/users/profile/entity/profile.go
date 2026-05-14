// Package entity holds the admin-user profile row. The profile is
// 1:1 with admin_users via admin_user_id, stored in its own table
// rather than on admin_users so auth-critical columns stay minimal
// and adding new profile fields doesn't churn the hot auth path.
package entity

import "time"

// AdminUserProfile mirrors one row of admin_user_profiles.
// AvatarAssetID is the opaque on-disk filename in the avatar store
// (empty string when no avatar is set). The serve path
// /p/admin-avatars/<admin_user_id> looks up this field to resolve
// the bytes.
type AdminUserProfile struct {
	AdminUserID   string    `json:"admin_user_id"`
	FirstName     string    `json:"first_name"`
	LastName      string    `json:"last_name"`
	Phone         string    `json:"phone"`
	AvatarAssetID string    `json:"avatar_asset_id"`
	Locale        string    `json:"locale"`
	Timezone      string    `json:"timezone"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

package entity

// ProfileField is one custom field in an organization's profile schema.
// AcceptMime and MaxBytes are only meaningful for DataTypeFile; they
// govern what the /profile/me file-upload endpoint accepts for this
// field. An empty AcceptMime means "use the endpoint default allowlist";
// a zero MaxBytes means "use the default cap (5 MB)".
type ProfileField struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Label       string   `json:"label"`
	Description string   `json:"description"`
	DataType    string   `json:"data_type"`
	IsRequired  bool     `json:"is_required"`
	MinValue    *int     `json:"min_value"`
	MaxValue    *int     `json:"max_value"`
	Options     []string `json:"options"`
	Regex       string   `json:"regex"`
	AcceptMime  string   `json:"accept_mime"`
	MaxBytes    int      `json:"max_bytes"`
	OrderIndex  int      `json:"order_index"`
	IsActive    bool     `json:"is_active"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

// UserProfile is one organization user's filled profile values.
type UserProfile struct {
	UserID      string                 `json:"user_id"`
	ProfileData map[string]interface{} `json:"profile_data"`
	UpdatedAt   string                 `json:"updated_at"`
}

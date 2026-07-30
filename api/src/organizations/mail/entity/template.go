package entity

// OrgMailTemplateResponse is one org-scoped email template — mirrors
// api/src/mail/template.Template field-for-field.
type OrgMailTemplateResponse struct {
	ID          string `json:"id" example:"b3202323-6270-448c-8001-6b803aca8053"`
	Name        string `json:"name" example:"user_invite"`
	Description string `json:"description" example:"Sent when a new org user is invited."`
	Subject     string `json:"subject" example:"Welcome to {{.OrgName}}"`
	TextBody    string `json:"text_body"`
	HTMLBody    string `json:"html_body"`
	IsActive    bool   `json:"is_active" example:"true"`
	CreatedAt   string `json:"created_at" example:"2026-07-30T10:00:00Z"`
	UpdatedAt   string `json:"updated_at" example:"2026-07-30T10:00:00Z"`
}

// OrgMailTemplateCreateRequest is the POST body for a new template.
type OrgMailTemplateCreateRequest struct {
	Name        string `json:"name" example:"user_invite"`
	Description string `json:"description"`
	Subject     string `json:"subject"`
	TextBody    string `json:"text_body"`
	HTMLBody    string `json:"html_body"`
	IsActive    bool   `json:"is_active" example:"true"`
}

// OrgMailTemplateUpdateRequest is the PATCH body — Name is immutable,
// mirrors template.Patch.
type OrgMailTemplateUpdateRequest struct {
	Description *string `json:"description,omitempty"`
	Subject     *string `json:"subject,omitempty"`
	TextBody    *string `json:"text_body,omitempty"`
	HTMLBody    *string `json:"html_body,omitempty"`
	IsActive    *bool   `json:"is_active,omitempty"`
}

type OrgMailTemplateListResponse struct {
	Items []OrgMailTemplateResponse `json:"items"`
	Total int                       `json:"total" example:"3"`
}

type OrgMailTemplateListSuccess struct {
	Success bool                        `json:"success" example:"true"`
	Message OrgMailTemplateListResponse `json:"message"`
}

type OrgMailTemplateSuccess struct {
	Success bool                    `json:"success" example:"true"`
	Message OrgMailTemplateResponse `json:"message"`
}

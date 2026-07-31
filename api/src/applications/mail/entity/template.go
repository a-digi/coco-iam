package entity

// AppMailTemplateResponse is one application-scoped email template —
// mirrors api/src/mail/template.Template field-for-field.
type AppMailTemplateResponse struct {
	ID          string `json:"id" example:"b3202323-6270-448c-8001-6b803aca8053"`
	Name        string `json:"name" example:"user_invite"`
	Description string `json:"description" example:"Sent when a new app user is invited."`
	Subject     string `json:"subject" example:"Welcome to {{.AppName}}"`
	TextBody    string `json:"text_body"`
	HTMLBody    string `json:"html_body"`
	IsActive    bool   `json:"is_active" example:"true"`
	CreatedAt   string `json:"created_at" example:"2026-07-30T10:00:00Z"`
	UpdatedAt   string `json:"updated_at" example:"2026-07-30T10:00:00Z"`
}

// AppMailTemplateCreateRequest is the POST body for a new template.
type AppMailTemplateCreateRequest struct {
	Name        string `json:"name" example:"user_invite"`
	Description string `json:"description"`
	Subject     string `json:"subject"`
	TextBody    string `json:"text_body"`
	HTMLBody    string `json:"html_body"`
	IsActive    bool   `json:"is_active" example:"true"`
}

// AppMailTemplateUpdateRequest is the PATCH body — Name is immutable,
// mirrors template.Patch.
type AppMailTemplateUpdateRequest struct {
	Description *string `json:"description,omitempty"`
	Subject     *string `json:"subject,omitempty"`
	TextBody    *string `json:"text_body,omitempty"`
	HTMLBody    *string `json:"html_body,omitempty"`
	IsActive    *bool   `json:"is_active,omitempty"`
}

type AppMailTemplateListResponse struct {
	Items []AppMailTemplateResponse `json:"items"`
	Total int                       `json:"total" example:"3"`
}

type AppMailTemplateListSuccess struct {
	Success bool                        `json:"success" example:"true"`
	Message AppMailTemplateListResponse `json:"message"`
}

type AppMailTemplateSuccess struct {
	Success bool                    `json:"success" example:"true"`
	Message AppMailTemplateResponse `json:"message"`
}

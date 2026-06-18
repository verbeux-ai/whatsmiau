package dto

type ListContactsRequest struct {
	InstanceID string `param:"instance" validate:"required"`
	Page       *int   `query:"page" validate:"required,min=1"`
	Limit      *int   `query:"limit" validate:"required,min=1,max=100"`
}

type ContactResponse struct {
	JID           string `json:"jid"`
	FirstName     string `json:"firstName,omitempty"`
	FullName      string `json:"fullName,omitempty"`
	PushName      string `json:"pushName,omitempty"`
	BusinessName  string `json:"businessName,omitempty"`
	RedactedPhone string `json:"redactedPhone,omitempty"`
	DisplayName   string `json:"displayName"`
}

type ContactsPaginationResponse struct {
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	Total      int `json:"total"`
	TotalPages int `json:"totalPages"`
}

type ListContactsResponse struct {
	Data       []ContactResponse          `json:"data"`
	Pagination ContactsPaginationResponse `json:"pagination"`
}

package dto

type ListContactsRequest struct {
	InstanceID string `param:"nameInstance" validate:"required"`
	Page       *int   `query:"page" validate:"required,min=1"`
	Limit      *int   `query:"limit" validate:"required,min=1,max=100"`
}

type ContactResponse struct {
	ID            string `json:"id"`
	RemoteJID     string `json:"remoteJid"`
	PushName      string `json:"pushName"`
	ProfilePicURL string `json:"profilePicUrl"`
	CreatedAt     string `json:"createdAt"`
	UpdatedAt     string `json:"updatedAt"`
	InstanceID    string `json:"instanceId"`
	IsGroup       bool   `json:"isGroup"`
	IsSaved       bool   `json:"isSaved"`
	Type          string `json:"type"`
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

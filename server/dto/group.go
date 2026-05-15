package dto

type CreateGroupRequest struct {
	InstanceID          string   `param:"instance" validate:"required" swaggerignore:"true"`
	Subject             string   `json:"subject" validate:"required,min=1,max=25"`
	Description         string   `json:"description,omitempty"`
	Participants        []string `json:"participants" validate:"omitempty,dive,required"`
	PromoteParticipants bool     `json:"promoteParticipants,omitempty"`
}

type GroupSubjectRequest struct {
	InstanceID string `param:"instance" validate:"required" swaggerignore:"true"`
	GroupJid   string `json:"groupJid" validate:"required"`
	Subject    string `json:"subject" validate:"required,min=1,max=25"`
}

type GroupPictureRequest struct {
	InstanceID string `param:"instance" validate:"required" swaggerignore:"true"`
	GroupJid   string `json:"groupJid" validate:"required"`
	// Image can be either a base64-encoded JPEG or a URL pointing to a JPEG.
	Image string `json:"image" validate:"required"`
}

type GroupDescriptionRequest struct {
	InstanceID  string `param:"instance" validate:"required" swaggerignore:"true"`
	GroupJid    string `json:"groupJid" validate:"required"`
	Description string `json:"description"`
}

type GroupJidQuery struct {
	InstanceID string `param:"instance" validate:"required" swaggerignore:"true"`
	GroupJid   string `query:"groupJid" validate:"required"`
}

type FetchAllGroupsQuery struct {
	InstanceID      string `param:"instance" validate:"required" swaggerignore:"true"`
	GetParticipants string `query:"getParticipants"`
}

type GroupInviteCodeQuery struct {
	InstanceID string `param:"instance" validate:"required" swaggerignore:"true"`
	InviteCode string `query:"inviteCode" validate:"required"`
}

type GroupSendInviteRequest struct {
	InstanceID  string   `param:"instance" validate:"required" swaggerignore:"true"`
	GroupJid    string   `json:"groupJid" validate:"required"`
	Description string   `json:"description,omitempty"`
	Numbers     []string `json:"numbers" validate:"required,min=1,dive,required"`
}

type GroupUpdateParticipantRequest struct {
	InstanceID   string   `param:"instance" validate:"required" swaggerignore:"true"`
	GroupJid     string   `json:"groupJid" validate:"required"`
	Action       string   `json:"action" validate:"required,oneof=add remove promote demote"`
	Participants []string `json:"participants" validate:"required,min=1,dive,required"`
}

type GroupUpdateSettingRequest struct {
	InstanceID string `param:"instance" validate:"required" swaggerignore:"true"`
	GroupJid   string `json:"groupJid" validate:"required"`
	Action     string `json:"action" validate:"required,oneof=announcement not_announcement locked unlocked"`
}

type GroupToggleEphemeralRequest struct {
	InstanceID string `param:"instance" validate:"required" swaggerignore:"true"`
	GroupJid   string `json:"groupJid" validate:"required"`
	Expiration uint32 `json:"expiration" validate:"oneof=0 86400 604800 7776000"`
}

type GroupRevokeInviteRequest struct {
	InstanceID string `param:"instance" validate:"required" swaggerignore:"true"`
	GroupJid   string `json:"groupJid" validate:"required"`
}

type GroupLeaveRequest struct {
	InstanceID string `param:"instance" validate:"required" swaggerignore:"true"`
	GroupJid   string `query:"groupJid" validate:"required"`
}

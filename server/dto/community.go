package dto

type CreateCommunityRequest struct {
	InstanceID  string `param:"instance" validate:"required" swaggerignore:"true"`
	Subject     string `json:"subject" validate:"required,min=1,max=25"`
	Description string `json:"description,omitempty"`
}

type CreateCommunitySubGroupRequest struct {
	InstanceID   string   `param:"instance" validate:"required" swaggerignore:"true"`
	Subject      string   `json:"subject" validate:"required,min=1,max=25"`
	ParentJid    string   `json:"parentJid" validate:"required"`
	Participants []string `json:"participants" validate:"omitempty,dive,required"`
}

type CommunityLinkGroupRequest struct {
	InstanceID string `param:"instance" validate:"required" swaggerignore:"true"`
	ParentJid  string `json:"parentJid" validate:"required"`
	ChildJid   string `json:"childJid" validate:"required"`
}

type CommunityJidQuery struct {
	InstanceID   string `param:"instance" validate:"required" swaggerignore:"true"`
	CommunityJid string `query:"communityJid" validate:"required"`
}

type CommunitySetJoinApprovalModeRequest struct {
	InstanceID   string `param:"instance" validate:"required" swaggerignore:"true"`
	CommunityJid string `json:"communityJid" validate:"required"`
	Mode         bool   `json:"mode"`
}

type CommunitySetMemberAddModeRequest struct {
	InstanceID   string `param:"instance" validate:"required" swaggerignore:"true"`
	CommunityJid string `json:"communityJid" validate:"required"`
	Mode         string `json:"mode" validate:"required,oneof=admin_add all_member_add"`
}

type CommunityUpdateRequestParticipantsRequest struct {
	InstanceID   string   `param:"instance" validate:"required" swaggerignore:"true"`
	CommunityJid string   `json:"communityJid" validate:"required"`
	Action       string   `json:"action" validate:"required,oneof=approve reject"`
	Participants []string `json:"participants" validate:"omitempty,dive,required"`
}

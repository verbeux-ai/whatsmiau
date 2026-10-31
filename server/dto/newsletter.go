package dto

type NewsletterInstanceRequest struct {
	InstanceID string `param:"instance" validate:"required" swaggerignore:"true"`
}

type NewsletterJidQuery struct {
	InstanceID    string `param:"instance" validate:"required" swaggerignore:"true"`
	NewsletterJid string `query:"newsletterJid" validate:"required"`
}

type NewsletterInviteQuery struct {
	InstanceID string `param:"instance" validate:"required" swaggerignore:"true"`
	// InviteCode accepts the bare code or a https://whatsapp.com/channel/ link.
	InviteCode string `query:"inviteCode" validate:"required"`
}

type CreateNewsletterRequest struct {
	InstanceID  string `param:"instance" validate:"required" swaggerignore:"true"`
	Name        string `json:"name" validate:"required" example:"Daily Deals"`
	Description string `json:"description,omitempty" example:"The best offers of the day"`
	// Picture can be an http(s) URL, a data: URI or raw base64 of an image (JPEG recommended).
	Picture string `json:"picture,omitempty"`
}

type NewsletterJidRequest struct {
	InstanceID    string `param:"instance" validate:"required" swaggerignore:"true"`
	NewsletterJid string `json:"newsletterJid" validate:"required" example:"120363144038483540@newsletter"`
}

type NewsletterActionResponse struct {
	NewsletterJid string `json:"newsletterJid" example:"120363144038483540@newsletter"`
	Action        string `json:"action" enums:"follow,unfollow,mute,unmute"`
}

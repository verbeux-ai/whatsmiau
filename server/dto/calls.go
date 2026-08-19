package dto

import "time"

// CallOfferRequest places a direct 1:1 audio call to Number or a WhatsApp JID.
type CallOfferRequest struct {
	Number string `json:"number" validate:"required"`
}

// CallOfferResponse preserves the JSON shape exposed by the current call API.
// The capitalized keys are intentional compatibility keys.
type CallOfferResponse struct {
	ID        string `json:"ID"`
	Recipient string `json:"Recipient"`
}

// CallSessionResponse preserves the safe lifecycle view returned by the call
// API. It deliberately excludes call keys, relay data, and media payloads.
type CallSessionResponse struct {
	ID        string `json:"ID"`
	Peer      string `json:"Peer"`
	Direction string `json:"Direction" enums:"incoming,outgoing"`
	Media     string `json:"Media" enums:"audio"`
	State     string `json:"State" enums:"calling,ringing,connecting,active,ended,idle"`
	Reason    string `json:"Reason"`

	CanAnswer bool `json:"CanAnswer"`
	CanReject bool `json:"CanReject"`
	CanHangup bool `json:"CanHangup"`

	CreatedAt time.Time `json:"CreatedAt"`
	UpdatedAt time.Time `json:"UpdatedAt"`
}

// CallActionResponse is returned after an accepted call control action.
type CallActionResponse struct {
	Status string `json:"status" example:"ok"`
}

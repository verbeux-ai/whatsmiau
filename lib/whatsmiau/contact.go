package whatsmiau

import (
	"context"
	"sort"

	"go.mau.fi/whatsmeow"
)

type ListContactsRequest struct{ InstanceID string }

type ContactResponse struct {
	JID           string `json:"jid"`
	Found         bool   `json:"found"`
	FirstName     string `json:"firstName,omitempty"`
	FullName      string `json:"fullName,omitempty"`
	PushName      string `json:"pushName,omitempty"`
	BusinessName  string `json:"businessName,omitempty"`
	RedactedPhone string `json:"redactedPhone,omitempty"`
}

// ListContacts returns the current Whatsmeow contact cache. It intentionally
// excludes profile pictures and any media so the manager can bootstrap an MCP
// dataset without moving binary or expiring URL data between services.
func (s *Whatsmiau) ListContacts(ctx context.Context, req *ListContactsRequest) ([]ContactResponse, error) {
	client, ok := s.clients.Load(req.InstanceID)
	if !ok || client.Store == nil || client.Store.Contacts == nil {
		return nil, whatsmeow.ErrClientIsNil
	}
	contacts, err := client.Store.Contacts.GetAllContacts(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]ContactResponse, 0, len(contacts))
	for jid, info := range contacts {
		result = append(result, ContactResponse{JID: jid.ToNonAD().String(), Found: info.Found, FirstName: info.FirstName, FullName: info.FullName, PushName: info.PushName, BusinessName: info.BusinessName, RedactedPhone: info.RedactedPhone})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].JID < result[j].JID })
	return result, nil
}

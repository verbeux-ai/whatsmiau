package whatsmiau

import (
	"context"
	"errors"
	"sort"
	"strings"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

var ErrInvalidPagination = errors.New("invalid pagination")

type ContactListItem struct {
	ID            string
	RemoteJID     string
	PushName      string
	ProfilePicURL string
	CreatedAt     string
	UpdatedAt     string
	InstanceID    string
	IsGroup       bool
	IsSaved       bool
	Type          string
}

type ContactListResult struct {
	Data       []ContactListItem
	Page       int
	Limit      int
	Total      int
	TotalPages int
}

func displayNameFromContact(jid string, info types.ContactInfo) string {
	switch {
	case strings.TrimSpace(info.FullName) != "":
		return info.FullName
	case strings.TrimSpace(info.BusinessName) != "":
		return info.BusinessName
	case strings.TrimSpace(info.PushName) != "":
		return info.PushName
	case strings.TrimSpace(info.FirstName) != "":
		return info.FirstName
	default:
		return jid
	}
}

func paginateContacts(items []ContactListItem, page, limit int) ContactListResult {
	total := len(items)
	totalPages := total / limit
	if total%limit != 0 {
		totalPages++
	}

	if page > totalPages && totalPages > 0 {
		return ContactListResult{
			Data:       []ContactListItem{},
			Page:       page,
			Limit:      limit,
			Total:      total,
			TotalPages: totalPages,
		}
	}

	offset := (page - 1) * limit
	if offset >= total {
		return ContactListResult{
			Data:       []ContactListItem{},
			Page:       page,
			Limit:      limit,
			Total:      total,
			TotalPages: totalPages,
		}
	}

	end := offset + limit
	if end > total {
		end = total
	}

	return ContactListResult{
		Data:       items[offset:end],
		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: totalPages,
	}
}

func (s *Whatsmiau) ListContacts(ctx context.Context, instanceID string, page, limit int) (*ContactListResult, error) {
	if page < 1 || limit < 1 || limit > 100 {
		return nil, ErrInvalidPagination
	}

	client, ok := s.clients.Load(instanceID)
	if !ok || client == nil {
		return nil, whatsmeow.ErrClientIsNil
	}
	if !client.IsLoggedIn() {
		return nil, whatsmeow.ErrClientIsNil
	}
	if client.Store == nil || client.Store.Contacts == nil {
		return nil, errors.New("contact store is unavailable")
	}

	contacts, err := client.Store.Contacts.GetAllContacts(ctx)
	if err != nil {
		return nil, err
	}

	type sortableContact struct {
		item             ContactListItem
		lowerDisplayName string
	}

	sortables := make([]sortableContact, 0, len(contacts))
	for jid, info := range contacts {
		jidValue := jid.ToNonAD().String()
		displayName := displayNameFromContact(jidValue, info)
		sortables = append(sortables, sortableContact{
			item: ContactListItem{
				ID:            jidValue,
				RemoteJID:     jidValue,
				PushName:      info.PushName,
				ProfilePicURL: "",
				CreatedAt:     "",
				UpdatedAt:     "",
				InstanceID:    instanceID,
				IsGroup:       false,
				IsSaved:       true,
				Type:          "contact",
			},
			lowerDisplayName: strings.ToLower(displayName),
		})
	}

	sort.Slice(sortables, func(i, j int) bool {
		if sortables[i].lowerDisplayName == sortables[j].lowerDisplayName {
			return sortables[i].item.RemoteJID < sortables[j].item.RemoteJID
		}
		return sortables[i].lowerDisplayName < sortables[j].lowerDisplayName
	})

	items := make([]ContactListItem, len(sortables))
	for i, sortable := range sortables {
		items[i] = sortable.item
	}

	result := paginateContacts(items, page, limit)
	return &result, nil
}

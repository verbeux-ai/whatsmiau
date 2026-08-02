package whatsmiau

import (
	"math"
	"testing"

	"go.mau.fi/whatsmeow/types"
)

func TestDisplayNameFromContactFallbackOrder(t *testing.T) {
	tests := []struct {
		name string
		info types.ContactInfo
		want string
	}{
		{
			name: "prefers full name",
			info: types.ContactInfo{
				FirstName:    "First",
				FullName:     "Full",
				PushName:     "Push",
				BusinessName: "Business",
			},
			want: "Full",
		},
		{
			name: "falls back to business name",
			info: types.ContactInfo{
				FirstName:    "First",
				PushName:     "Push",
				BusinessName: "Business",
			},
			want: "Business",
		},
		{
			name: "falls back to push name",
			info: types.ContactInfo{
				FirstName: "First",
				PushName:  "Push",
			},
			want: "Push",
		},
		{
			name: "falls back to first name",
			info: types.ContactInfo{
				FirstName: "First",
			},
			want: "First",
		},
		{
			name: "falls back to jid",
			info: types.ContactInfo{},
			want: "1@s.whatsapp.net",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := displayNameFromContact("1@s.whatsapp.net", tt.info)
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestPaginateContacts(t *testing.T) {
	items := []ContactListItem{
		{ID: "1@s.whatsapp.net", RemoteJID: "1@s.whatsapp.net", PushName: "A"},
		{ID: "2@s.whatsapp.net", RemoteJID: "2@s.whatsapp.net", PushName: "B"},
		{ID: "3@s.whatsapp.net", RemoteJID: "3@s.whatsapp.net", PushName: "C"},
	}

	result := paginateContacts(items, 2, 2)
	if result.Total != 3 {
		t.Fatalf("expected total 3, got %d", result.Total)
	}
	if result.TotalPages != 2 {
		t.Fatalf("expected totalPages 2, got %d", result.TotalPages)
	}
	if len(result.Data) != 1 {
		t.Fatalf("expected one item on second page, got %d", len(result.Data))
	}
	if result.Data[0].RemoteJID != "3@s.whatsapp.net" {
		t.Fatalf("expected last item on second page, got %q", result.Data[0].RemoteJID)
	}
}

func TestPaginateContactsOutOfRange(t *testing.T) {
	items := []ContactListItem{{ID: "1@s.whatsapp.net", RemoteJID: "1@s.whatsapp.net", PushName: "A"}}

	result := paginateContacts(items, 2, 1)
	if len(result.Data) != 0 {
		t.Fatalf("expected empty page, got %d items", len(result.Data))
	}
	if result.Total != 1 {
		t.Fatalf("expected total 1, got %d", result.Total)
	}
	if result.TotalPages != 1 {
		t.Fatalf("expected totalPages 1, got %d", result.TotalPages)
	}
}

func TestPaginateContactsWithHugePageDoesNotOverflow(t *testing.T) {
	items := []ContactListItem{{ID: "1@s.whatsapp.net", RemoteJID: "1@s.whatsapp.net", PushName: "A"}}

	result := paginateContacts(items, math.MaxInt, 1)
	if len(result.Data) != 0 {
		t.Fatalf("expected empty page, got %d items", len(result.Data))
	}
	if result.Page != math.MaxInt {
		t.Fatalf("expected page to be preserved, got %d", result.Page)
	}
	if result.Total != 1 {
		t.Fatalf("expected total 1, got %d", result.Total)
	}
	if result.TotalPages != 1 {
		t.Fatalf("expected totalPages 1, got %d", result.TotalPages)
	}
}

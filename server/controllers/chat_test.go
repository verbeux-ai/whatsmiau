package controllers

import (
	"reflect"
	"testing"

	"github.com/verbeux-ai/whatsmiau/server/dto"
)

func TestGroupReadMessagesKeepsPlayedApart(t *testing.T) {
	// A voice note marked played and a text marked read share the chat but
	// not the receipt type: merging them would send one of the two wrong.
	got := groupReadMessages([]dto.ReadMessagesRequestItem{
		{RemoteJid: "5511999999999@s.whatsapp.net", ID: "A", Played: true},
		{RemoteJid: "5511999999999@s.whatsapp.net", ID: "B"},
		{RemoteJid: "5511999999999@s.whatsapp.net", ID: "C", Played: true},
	})

	want := []readBatch{
		{remoteJid: "5511999999999@s.whatsapp.net", played: true, ids: []string{"A", "C"}},
		{remoteJid: "5511999999999@s.whatsapp.net", played: false, ids: []string{"B"}},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("groupReadMessages() = %+v, want %+v", got, want)
	}
}

func TestGroupReadMessagesKeepsSendersApart(t *testing.T) {
	// Inside a group the receipt has to name who sent each message, so two
	// senders in the same chat cannot share a call.
	got := groupReadMessages([]dto.ReadMessagesRequestItem{
		{RemoteJid: "123@g.us", Sender: "5511888888888@s.whatsapp.net", ID: "A"},
		{RemoteJid: "123@g.us", Sender: "5511777777777@s.whatsapp.net", ID: "B"},
		{RemoteJid: "123@g.us", Sender: "5511888888888@s.whatsapp.net", ID: "C"},
	})

	if len(got) != 2 {
		t.Fatalf("groupReadMessages() produced %d batches, want 2", len(got))
	}
	if !reflect.DeepEqual(got[0].ids, []string{"A", "C"}) {
		t.Fatalf("first batch ids = %v, want [A C]", got[0].ids)
	}
	if !reflect.DeepEqual(got[1].ids, []string{"B"}) {
		t.Fatalf("second batch ids = %v, want [B]", got[1].ids)
	}
}

func TestGroupReadMessagesBatchesSameChat(t *testing.T) {
	// The reason this grouping exists: one call per chat when nothing forces
	// them apart.
	got := groupReadMessages([]dto.ReadMessagesRequestItem{
		{RemoteJid: "5511999999999@s.whatsapp.net", ID: "A"},
		{RemoteJid: "5511999999999@s.whatsapp.net", ID: "B"},
	})

	if len(got) != 1 {
		t.Fatalf("groupReadMessages() produced %d batches, want 1", len(got))
	}
	if !reflect.DeepEqual(got[0].ids, []string{"A", "B"}) {
		t.Fatalf("ids = %v, want [A B]", got[0].ids)
	}
}

func TestGroupReadMessagesEmpty(t *testing.T) {
	if got := groupReadMessages(nil); len(got) != 0 {
		t.Fatalf("groupReadMessages(nil) = %+v, want empty", got)
	}
}

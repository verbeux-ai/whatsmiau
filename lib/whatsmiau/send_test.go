package whatsmiau

import (
	"testing"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

func TestBuildSendTextMessageWithoutQuote(t *testing.T) {
	msg := buildSendTextMessage(&SendText{Text: "hello"}, nil, false)

	if msg.Conversation == nil || msg.GetConversation() != "hello" {
		t.Fatalf("expected Conversation=%q, got %v", "hello", msg.Conversation)
	}
	if msg.ExtendedTextMessage != nil {
		t.Fatalf("expected no ExtendedTextMessage, got %v", msg.ExtendedTextMessage)
	}
}

func TestBuildSendTextMessageQuoteIDOnly(t *testing.T) {
	msg := buildSendTextMessage(&SendText{Text: "reply", Quote: &Quote{MessageID: "ABC123"}}, nil, false)

	ext := msg.ExtendedTextMessage
	if ext == nil {
		t.Fatal("expected ExtendedTextMessage for quoted message")
	}
	if ext.GetText() != "reply" {
		t.Fatalf("expected text=%q, got %q", "reply", ext.GetText())
	}
	if ext.ContextInfo == nil {
		t.Fatal("expected ContextInfo")
	}
	if ext.ContextInfo.GetStanzaID() != "ABC123" {
		t.Fatalf("expected stanzaID=ABC123, got %q", ext.ContextInfo.GetStanzaID())
	}
	if ext.ContextInfo.Participant != nil {
		t.Fatalf("expected no participant, got %q", ext.ContextInfo.GetParticipant())
	}
	if ext.ContextInfo.QuotedMessage != nil {
		t.Fatalf("expected no quotedMessage, got %v", ext.ContextInfo.QuotedMessage)
	}
}

func TestBuildSendTextMessageQuoteWithConversation(t *testing.T) {
	msg := buildSendTextMessage(&SendText{
		Text:  "reply",
		Quote: &Quote{MessageID: "ABC123", Message: "original text"},
	}, nil, false)

	if msg.ExtendedTextMessage == nil {
		t.Fatal("expected ExtendedTextMessage for quoted message")
	}
	ci := msg.ExtendedTextMessage.ContextInfo
	if ci.GetStanzaID() != "ABC123" {
		t.Fatalf("expected stanzaID=ABC123, got %q", ci.GetStanzaID())
	}
	if ci.QuotedMessage == nil || ci.QuotedMessage.GetConversation() != "original text" {
		t.Fatalf("expected quotedMessage conversation=%q, got %v", "original text", ci.QuotedMessage)
	}
}

func TestBuildSendTextMessageQuoteWithParticipant(t *testing.T) {
	participant, err := types.ParseJID("5511999999999@s.whatsapp.net")
	if err != nil {
		t.Fatalf("failed to parse participant JID: %v", err)
	}

	msg := buildSendTextMessage(&SendText{
		Text:  "reply",
		Quote: &Quote{MessageID: "ABC123", Message: "original text", Participant: &participant},
	}, nil, false)

	if msg.ExtendedTextMessage == nil {
		t.Fatal("expected ExtendedTextMessage for quoted message")
	}
	ci := msg.ExtendedTextMessage.ContextInfo
	if ci.GetStanzaID() != "ABC123" {
		t.Fatalf("expected stanzaID=ABC123, got %q", ci.GetStanzaID())
	}
	if ci.GetParticipant() != participant.String() {
		t.Fatalf("expected participant=%q, got %q", participant.String(), ci.GetParticipant())
	}
	if ci.QuotedMessage == nil {
		t.Fatal("expected quotedMessage")
	}
}

func TestBuildSendTextMessageQuoteTextPreserved(t *testing.T) {
	// The reply text must go into ExtendedTextMessage.Text, not Conversation.
	msg := buildSendTextMessage(&SendText{Text: "reply", Quote: &Quote{MessageID: "ABC123"}}, nil, false)

	if msg.GetConversation() != "" {
		t.Fatalf("expected no Conversation field, got %q", msg.GetConversation())
	}
	if msg.ExtendedTextMessage.GetText() != "reply" {
		t.Fatalf("expected text=%q, got %q", "reply", msg.ExtendedTextMessage.GetText())
	}
	if !proto.Equal(msg, buildSendTextMessage(&SendText{Text: "reply", Quote: &Quote{MessageID: "ABC123"}}, nil, false)) {
		t.Fatal("expected deterministic message construction")
	}
}

func TestBuildContextInfoNoQuote(t *testing.T) {
	if ci := buildContextInfo(nil, nil, false); ci != nil {
		t.Fatalf("expected nil ContextInfo, got %v", ci)
	}
	if ci := buildContextInfo(&Quote{}, nil, false); ci != nil {
		t.Fatalf("expected nil ContextInfo for empty quote, got %v", ci)
	}
}

func TestBuildContextInfoQuoteIDOnly(t *testing.T) {
	ci := buildContextInfo(&Quote{MessageID: "ABC123"}, nil, false)

	if ci.GetStanzaID() != "ABC123" {
		t.Fatalf("expected stanzaID=ABC123, got %q", ci.GetStanzaID())
	}
	if ci.Participant != nil {
		t.Fatalf("expected no participant, got %q", ci.GetParticipant())
	}
	if ci.QuotedMessage != nil {
		t.Fatalf("expected no quotedMessage, got %v", ci.QuotedMessage)
	}
}

func TestBuildContextInfoWithContentAndParticipant(t *testing.T) {
	participant, err := types.ParseJID("5511999999999@s.whatsapp.net")
	if err != nil {
		t.Fatalf("failed to parse participant JID: %v", err)
	}

	ci := buildContextInfo(&Quote{
		MessageID:   "ABC123",
		Message:     "original text",
		Participant: &participant,
	}, nil, false)

	if ci.GetStanzaID() != "ABC123" {
		t.Fatalf("expected stanzaID=ABC123, got %q", ci.GetStanzaID())
	}
	if ci.GetParticipant() != participant.String() {
		t.Fatalf("expected participant=%q, got %q", participant.String(), ci.GetParticipant())
	}
	if ci.QuotedMessage == nil || ci.QuotedMessage.GetConversation() != "original text" {
		t.Fatalf("expected quotedMessage conversation=%q, got %v", "original text", ci.QuotedMessage)
	}
}

func TestBuildContactMessageWithQuote(t *testing.T) {
	// Mirrors SendContact's single-contact branch (send_more.go): the ContextInfo
	// must be attached to the ContactMessage and survive proto marshaling.
	contact := contactToProto(SendContactItem{FullName: "Ivo Silva", PhoneNumber: "5511999997777"})
	contact.ContextInfo = buildContextInfo(&Quote{MessageID: "ABC123", Message: "h"}, nil, false)
	msg := &waE2E.Message{ContactMessage: contact}

	ci := msg.ContactMessage.GetContextInfo()
	if ci == nil {
		t.Fatal("expected ContextInfo on ContactMessage")
	}
	if ci.GetStanzaID() != "ABC123" {
		t.Fatalf("expected stanzaID=ABC123, got %q", ci.GetStanzaID())
	}
	if ci.QuotedMessage == nil || ci.QuotedMessage.GetConversation() != "h" {
		t.Fatalf("expected quotedMessage conversation=%q, got %v", "h", ci.QuotedMessage)
	}

	if _, err := proto.Marshal(msg); err != nil {
		t.Fatalf("expected message to marshal, got %v", err)
	}
}

func TestBuildSendTextMessageWithMentions(t *testing.T) {
	msg := buildSendTextMessage(&SendText{Text: "hello", Mentioned: []string{"5511999999999"}}, []string{"5511999999999@s.whatsapp.net"}, false)

	if msg.Conversation != nil {
		t.Fatalf("expected no Conversation field when mentioning, got %v", msg.Conversation)
	}
	ext := msg.ExtendedTextMessage
	if ext == nil {
		t.Fatal("expected ExtendedTextMessage when mentioning")
	}
	if ext.GetText() != "hello" {
		t.Fatalf("expected text=%q, got %q", "hello", ext.GetText())
	}
	ci := ext.ContextInfo
	if ci == nil {
		t.Fatal("expected ContextInfo when mentioning")
	}
	if len(ci.GetMentionedJID()) != 1 || ci.GetMentionedJID()[0] != "5511999999999@s.whatsapp.net" {
		t.Fatalf("expected mentionedJID=[5511999999999@s.whatsapp.net], got %v", ci.GetMentionedJID())
	}
}

func TestBuildSendTextMessageMentionsWithQuote(t *testing.T) {
	msg := buildSendTextMessage(
		&SendText{Text: "reply", Quote: &Quote{MessageID: "ABC123"}, Mentioned: []string{"5511999999999"}},
		[]string{"5511999999999@s.whatsapp.net"},
		false,
	)

	ext := msg.ExtendedTextMessage
	if ext == nil {
		t.Fatal("expected ExtendedTextMessage")
	}
	ci := ext.ContextInfo
	if ci.GetStanzaID() != "ABC123" {
		t.Fatalf("expected stanzaID=ABC123, got %q", ci.GetStanzaID())
	}
	if len(ci.GetMentionedJID()) != 1 || ci.GetMentionedJID()[0] != "5511999999999@s.whatsapp.net" {
		t.Fatalf("expected mentionedJID=[5511999999999@s.whatsapp.net], got %v", ci.GetMentionedJID())
	}
}

func TestBuildContextInfoMentionsOnly(t *testing.T) {
	ci := buildContextInfo(nil, []string{"5511999999999@s.whatsapp.net", "5511888888888@s.whatsapp.net"}, false)
	if ci == nil {
		t.Fatal("expected non-nil ContextInfo for mentions")
	}
	if len(ci.GetMentionedJID()) != 2 {
		t.Fatalf("expected 2 mentioned JIDs, got %v", ci.GetMentionedJID())
	}
	if ci.StanzaID != nil {
		t.Fatalf("expected no stanzaID, got %q", ci.GetStanzaID())
	}
}

func TestResolveMentionsIndividual(t *testing.T) {
	s := &Whatsmiau{}
	remoteJID, err := types.ParseJID("120363000000000000@g.us")
	if err != nil {
		t.Fatalf("failed to parse group JID: %v", err)
	}
	mentioned, everyone, err := s.resolveMentions(remoteJID, false, []string{"5511999999999", "5511888888888@s.whatsapp.net"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if everyone {
		t.Fatal("expected everyone=false for individual mentions")
	}
	expected := []string{"5511999999999@s.whatsapp.net", "5511888888888@s.whatsapp.net"}
	if len(mentioned) != 2 || mentioned[0] != expected[0] || mentioned[1] != expected[1] {
		t.Fatalf("expected %v, got %v", expected, mentioned)
	}
}

func TestResolveMentionsEveryOneNonGroupIgnored(t *testing.T) {
	s := &Whatsmiau{}
	remoteJID, err := types.ParseJID("5511999999999@s.whatsapp.net")
	if err != nil {
		t.Fatalf("failed to parse JID: %v", err)
	}
	mentioned, everyone, err := s.resolveMentions(remoteJID, true, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if everyone {
		t.Fatal("expected everyone=false for non-group destination")
	}
	if len(mentioned) != 0 {
		t.Fatalf("expected no mentions for non-group, got %v", mentioned)
	}
}

func TestResolveMentionsEveryOneGroup(t *testing.T) {
	s := &Whatsmiau{}
	remoteJID, err := types.ParseJID("120363000000000000@g.us")
	if err != nil {
		t.Fatalf("failed to parse group JID: %v", err)
	}
	mentioned, everyone, err := s.resolveMentions(remoteJID, true, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !everyone {
		t.Fatal("expected everyone=true for group destination")
	}
	if len(mentioned) != 0 {
		t.Fatalf("expected no MentionedJID for everyone, got %v", mentioned)
	}
}

func TestResolveMentionsEveryOneWithIndividualMentions(t *testing.T) {
	s := &Whatsmiau{}
	remoteJID, err := types.ParseJID("120363000000000000@g.us")
	if err != nil {
		t.Fatalf("failed to parse group JID: %v", err)
	}
	mentioned, everyone, err := s.resolveMentions(remoteJID, true, []string{"5511999999999", "5511888888888@lid"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !everyone {
		t.Fatal("expected everyone=true for group destination")
	}
	if len(mentioned) != 2 {
		t.Fatalf("expected 2 mentioned JIDs alongside everyone, got %v", mentioned)
	}
}

func TestBuildSendTextMessageEveryone(t *testing.T) {
	msg := buildSendTextMessage(&SendText{Text: "@all test"}, nil, true)

	if msg.Conversation != nil {
		t.Fatalf("expected no Conversation field when mentioning everyone, got %v", msg.Conversation)
	}
	ext := msg.ExtendedTextMessage
	if ext == nil {
		t.Fatal("expected ExtendedTextMessage when mentioning everyone")
	}
	if ext.GetText() != "@all test" {
		t.Fatalf("expected text=%q, got %q", "@all test", ext.GetText())
	}
	ci := ext.ContextInfo
	if ci == nil {
		t.Fatal("expected ContextInfo when mentioning everyone")
	}
	if ci.GetNonJIDMentions() != 1 {
		t.Fatalf("expected NonJIDMentions=1, got %d", ci.GetNonJIDMentions())
	}
	if len(ci.GetMentionedJID()) != 0 {
		t.Fatalf("expected no MentionedJID for everyone, got %v", ci.GetMentionedJID())
	}
}

func TestBuildContextInfoEveryoneWithQuote(t *testing.T) {
	ci := buildContextInfo(&Quote{MessageID: "ABC123"}, nil, true)
	if ci == nil {
		t.Fatal("expected non-nil ContextInfo")
	}
	if ci.GetStanzaID() != "ABC123" {
		t.Fatalf("expected stanzaID=ABC123, got %q", ci.GetStanzaID())
	}
	if ci.GetNonJIDMentions() != 1 {
		t.Fatalf("expected NonJIDMentions=1, got %d", ci.GetNonJIDMentions())
	}
}

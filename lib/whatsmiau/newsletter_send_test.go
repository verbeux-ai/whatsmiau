package whatsmiau

import (
	"context"
	"errors"
	"testing"

	"github.com/puzpuzpuz/xsync/v4"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

const testNewsletterJID = "120363144038483540@newsletter"

func mustParseJID(t *testing.T, raw string) types.JID {
	t.Helper()
	jid, err := types.ParseJID(raw)
	if err != nil {
		t.Fatalf("failed to parse JID %q: %v", raw, err)
	}
	return jid
}

// fakeUploader records which upload endpoint was used.
type fakeUploader struct {
	called string
}

func (f *fakeUploader) Upload(_ context.Context, data []byte, _ whatsmeow.MediaType) (whatsmeow.UploadResponse, error) {
	f.called = "upload"
	return whatsmeow.UploadResponse{
		DirectPath:    "/enc",
		MediaKey:      []byte("key"),
		FileEncSHA256: []byte("enc"),
		FileLength:    uint64(len(data)),
	}, nil
}

func (f *fakeUploader) UploadNewsletter(_ context.Context, data []byte, _ whatsmeow.MediaType) (whatsmeow.UploadResponse, error) {
	f.called = "newsletter"
	return whatsmeow.UploadResponse{
		DirectPath: "/plain",
		Handle:     "handle-1",
		FileLength: uint64(len(data)),
	}, nil
}

func TestUploadMediaUsesNewsletterUploadForChannels(t *testing.T) {
	up := &fakeUploader{}
	resp, err := uploadMedia(context.Background(), up, mustParseJID(t, testNewsletterJID), []byte("img"), whatsmeow.MediaImage)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if up.called != "newsletter" {
		t.Fatalf("expected the newsletter upload, got %q", up.called)
	}
	if resp.MediaKey != nil || resp.FileEncSHA256 != nil {
		t.Fatalf("channel media must not carry encryption fields, got key=%v enc=%v", resp.MediaKey, resp.FileEncSHA256)
	}
}

func TestUploadMediaKeepsEncryptedUploadForOtherChats(t *testing.T) {
	for _, raw := range []string{
		"5511999999999@s.whatsapp.net",
		"120363000000000000@g.us",
		"123456789012345@lid",
		"status@broadcast",
	} {
		up := &fakeUploader{}
		if _, err := uploadMedia(context.Background(), up, mustParseJID(t, raw), []byte("img"), whatsmeow.MediaImage); err != nil {
			t.Fatalf("%s: unexpected error: %v", raw, err)
		}
		if up.called != "upload" {
			t.Errorf("%s: expected the encrypted upload, got %q", raw, up.called)
		}
	}
}

func TestMediaSendExtraPassesHandleOnlyToChannels(t *testing.T) {
	uploaded := whatsmeow.UploadResponse{Handle: "handle-1"}

	extra := mediaSendExtra(mustParseJID(t, testNewsletterJID), uploaded)
	if len(extra) != 1 || extra[0].MediaHandle != "handle-1" {
		t.Fatalf("expected one extra with the media handle, got %+v", extra)
	}

	if extra := mediaSendExtra(mustParseJID(t, "120363000000000000@g.us"), uploaded); extra != nil {
		t.Fatalf("expected no extra for a group, got %+v", extra)
	}
	if extra := mediaSendExtra(mustParseJID(t, "5511999999999@s.whatsapp.net"), uploaded); extra != nil {
		t.Fatalf("expected no extra for a contact, got %+v", extra)
	}
	if extra := mediaSendExtra(mustParseJID(t, testNewsletterJID), whatsmeow.UploadResponse{}); extra != nil {
		t.Fatalf("expected no extra without a handle, got %+v", extra)
	}
}

func TestQuoteForChatDropsQuoteOnlyForChannels(t *testing.T) {
	q := &Quote{MessageID: "ABC123", Message: "original"}

	if got := quoteForChat(mustParseJID(t, testNewsletterJID), q); got != nil {
		t.Fatalf("expected no quote for a channel, got %+v", got)
	}
	if got := quoteForChat(mustParseJID(t, "120363000000000000@g.us"), q); got != q {
		t.Fatalf("expected the quote kept for a group, got %+v", got)
	}
}

func TestResolveMentionsIgnoredForChannels(t *testing.T) {
	s := &Whatsmiau{}
	// Even an invalid mention is ignored: channels have no mentions at all.
	jids, everyone, err := s.resolveMentions(mustParseJID(t, testNewsletterJID), true, []string{"5511999999999", "bad@@jid"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if jids != nil || everyone {
		t.Fatalf("expected no mentions for a channel, got jids=%v everyone=%v", jids, everyone)
	}
}

func TestBuildSendTextMessageForChannelIsPlainConversation(t *testing.T) {
	s := &Whatsmiau{}
	to := mustParseJID(t, testNewsletterJID)
	data := &SendText{
		Text:      "hello channel",
		RemoteJID: &to,
		Quote:     quoteForChat(to, &Quote{MessageID: "ABC123", Message: "original"}),
	}
	mentioned, everyone, err := s.resolveMentions(to, true, []string{"5511999999999"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	msg := buildSendTextMessage(data, mentioned, everyone)
	if msg.GetConversation() != "hello channel" {
		t.Fatalf("expected Conversation=%q, got %v", "hello channel", msg)
	}
	if msg.ExtendedTextMessage != nil {
		t.Fatalf("expected no ContextInfo carrier for a channel, got %v", msg.ExtendedTextMessage)
	}
}

func TestResolveJIDKeepsChannelJID(t *testing.T) {
	s := &Whatsmiau{}
	jid := mustParseJID(t, testNewsletterJID)
	// Non-phone JIDs return before the client is used.
	if got := s.resolveJID(context.Background(), nil, jid); got != jid {
		t.Fatalf("expected %s unchanged, got %s", jid, got)
	}
}

func TestRejectNewsletter(t *testing.T) {
	if err := rejectNewsletter(mustParseJID(t, testNewsletterJID), "list message"); !errors.Is(err, ErrNewsletterUnsupported) {
		t.Fatalf("expected ErrNewsletterUnsupported, got %v", err)
	}
	for _, raw := range []string{"120363000000000000@g.us", "5511999999999@s.whatsapp.net", "status@broadcast"} {
		if err := rejectNewsletter(mustParseJID(t, raw), "list message"); err != nil {
			t.Errorf("%s: expected no error, got %v", raw, err)
		}
	}
}

// newChannelTestService stores an idle client: every path below must return
// before using it.
func newChannelTestService() *Whatsmiau {
	s := &Whatsmiau{clients: xsync.NewMap[string, *whatsmeow.Client]()}
	s.clients.Store("instance", &whatsmeow.Client{})
	return s
}

func TestUnsupportedMessageTypesAreRejectedForChannels(t *testing.T) {
	s := newChannelTestService()
	ctx := context.Background()
	to := mustParseJID(t, testNewsletterJID)

	cases := map[string]func() error{
		"list": func() error {
			_, err := s.SendList(ctx, &SendListRequest{InstanceID: "instance", RemoteJID: &to})
			return err
		},
		"buttons": func() error {
			_, err := s.SendButtons(ctx, &SendButtonsRequestData{InstanceID: "instance", RemoteJID: &to})
			return err
		},
		"pix": func() error {
			_, err := s.SendPixPayment(ctx, &SendPixPaymentRequest{InstanceID: "instance", RemoteJID: &to})
			return err
		},
		"reaction": func() error {
			_, err := s.SendReaction(ctx, &SendReactionRequest{InstanceID: "instance", RemoteJID: &to, MessageID: "ABC123", Reaction: "\U0001F44D"})
			return err
		},
		"location": func() error {
			_, err := s.SendLocation(ctx, &SendLocationRequest{InstanceID: "instance", RemoteJID: &to})
			return err
		},
		"contact": func() error {
			_, err := s.SendContact(ctx, &SendContactRequest{InstanceID: "instance", RemoteJID: &to, Contacts: []SendContactItem{{FullName: "A"}}})
			return err
		},
	}
	for name, send := range cases {
		if err := send(); !errors.Is(err, ErrNewsletterUnsupported) {
			t.Errorf("%s: expected ErrNewsletterUnsupported, got %v", name, err)
		}
	}
}

func TestChatPresenceSkipsChannels(t *testing.T) {
	s := newChannelTestService()
	to := mustParseJID(t, testNewsletterJID)
	if err := s.ChatPresence(&ChatPresenceRequest{InstanceID: "instance", RemoteJID: &to, Presence: types.ChatPresenceComposing}); err != nil {
		t.Fatalf("expected presence to be skipped for a channel, got %v", err)
	}
}

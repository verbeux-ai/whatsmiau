package whatsmiau

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/puzpuzpuz/xsync/v4"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

type fakeNewsletterClient struct {
	subscribed    []*types.NewsletterMetadata
	info          *types.NewsletterMetadata
	infoErr       error
	created       *types.NewsletterMetadata
	createErr     error
	inviteKey     string
	createParams  whatsmeow.CreateNewsletterParams
	infoCallCount int
}

func (f *fakeNewsletterClient) GetSubscribedNewsletters(context.Context) ([]*types.NewsletterMetadata, error) {
	return f.subscribed, f.infoErr
}

func (f *fakeNewsletterClient) GetNewsletterInfo(context.Context, types.JID) (*types.NewsletterMetadata, error) {
	f.infoCallCount++
	return f.info, f.infoErr
}

func (f *fakeNewsletterClient) GetNewsletterInfoWithInvite(_ context.Context, key string) (*types.NewsletterMetadata, error) {
	f.inviteKey = key
	return f.info, f.infoErr
}

func (f *fakeNewsletterClient) CreateNewsletter(_ context.Context, params whatsmeow.CreateNewsletterParams) (*types.NewsletterMetadata, error) {
	f.createParams = params
	return f.created, f.createErr
}

func (f *fakeNewsletterClient) FollowNewsletter(context.Context, types.JID) error   { return nil }
func (f *fakeNewsletterClient) UnfollowNewsletter(context.Context, types.JID) error { return nil }
func (f *fakeNewsletterClient) NewsletterToggleMute(context.Context, types.JID, bool) error {
	return nil
}

var testNewsletterJID = types.NewJID("120363144038483540", types.NewsletterServer)

func fullNewsletterMetadata(role types.NewsletterRole, mute types.NewsletterMuteState) *types.NewsletterMetadata {
	meta := &types.NewsletterMetadata{
		ID:    testNewsletterJID,
		State: types.WrappedNewsletterState{Type: types.NewsletterStateActive},
		ThreadMeta: types.NewsletterThreadMetadata{
			InviteCode:        "0029Va4K0PZ5a245NkngBA2M",
			Name:              types.NewsletterText{Text: "Daily Deals"},
			Description:       types.NewsletterText{Text: "Offers"},
			SubscriberCount:   1234,
			VerificationState: types.NewsletterVerificationStateVerified,
			Picture:           &types.ProfilePictureInfo{ID: "pic-1", URL: "https://mmg.whatsapp.net/full.jpg", DirectPath: "/v/full.jpg"},
			Preview:           types.ProfilePictureInfo{URL: "https://mmg.whatsapp.net/preview.jpg"},
			Settings:          types.NewsletterSettings{ReactionCodes: types.NewsletterReactionSettings{Value: types.NewsletterReactionsModeBasic}},
		},
		ViewerMeta: &types.NewsletterViewerMetadata{Role: role, Mute: mute},
	}
	meta.ThreadMeta.CreationTime.Time = time.Unix(1700000000, 0)
	return meta
}

func TestBuildNewsletterResponseMapsEveryField(t *testing.T) {
	resp := buildNewsletterResponse(fullNewsletterMetadata(types.NewsletterRoleOwner, types.NewsletterMuteOn))

	if resp.Id != "120363144038483540@newsletter" || resp.Name != "Daily Deals" || resp.Description != "Offers" {
		t.Fatalf("identity fields = %+v", resp)
	}
	if resp.InviteCode != "0029Va4K0PZ5a245NkngBA2M" || resp.InviteURL != "https://whatsapp.com/channel/0029Va4K0PZ5a245NkngBA2M" {
		t.Fatalf("invite fields = %q %q", resp.InviteCode, resp.InviteURL)
	}
	if resp.PictureID != "pic-1" || resp.PictureURL != "https://mmg.whatsapp.net/full.jpg" || resp.PictureDirectPath != "/v/full.jpg" || resp.PreviewURL != "https://mmg.whatsapp.net/preview.jpg" {
		t.Fatalf("picture fields = %+v", resp)
	}
	if resp.SubscribersCount != 1234 || !resp.Verified || resp.State != "active" || resp.Creation != 1700000000 || resp.ReactionsMode != "basic" {
		t.Fatalf("metadata fields = %+v", resp)
	}
	if resp.Role == nil || *resp.Role != "owner" || resp.Muted == nil || !*resp.Muted || resp.CanPublish == nil || !*resp.CanPublish {
		t.Fatalf("viewer fields = role:%v muted:%v canPublish:%v", resp.Role, resp.Muted, resp.CanPublish)
	}
}

func TestBuildNewsletterResponseCanPublishFollowsRole(t *testing.T) {
	for role, want := range map[types.NewsletterRole]bool{
		types.NewsletterRoleOwner:      true,
		types.NewsletterRoleAdmin:      true,
		types.NewsletterRoleSubscriber: false,
		types.NewsletterRoleGuest:      false,
	} {
		resp := buildNewsletterResponse(fullNewsletterMetadata(role, types.NewsletterMuteOff))
		if resp.CanPublish == nil || *resp.CanPublish != want {
			t.Errorf("role %s canPublish = %v, want %v", role, resp.CanPublish, want)
		}
		if resp.Muted == nil || *resp.Muted {
			t.Errorf("role %s muted = %v, want false", role, resp.Muted)
		}
	}
}

func TestBuildNewsletterResponseHandlesMissingOptionalMetadata(t *testing.T) {
	resp := buildNewsletterResponse(&types.NewsletterMetadata{
		ID: testNewsletterJID,
		ThreadMeta: types.NewsletterThreadMetadata{
			Name:              types.NewsletterText{Text: "Bare"},
			VerificationState: types.NewsletterVerificationStateUnverified,
		},
	})

	if resp.PictureURL != "" || resp.PictureID != "" || resp.PreviewURL != "" || resp.InviteURL != "" || resp.Creation != 0 {
		t.Fatalf("absent metadata produced values: %+v", resp)
	}
	if resp.Verified {
		t.Fatal("unverified channel reported as verified")
	}
	if resp.Role != nil || resp.Muted != nil || resp.CanPublish != nil {
		t.Fatalf("viewer fields must be nil without viewer metadata: %+v", resp)
	}

	encoded, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	for _, key := range []string{"role", "muted", "canPublish"} {
		value, ok := decoded[key]
		if !ok || value != nil {
			t.Errorf("%s = %v (present=%v), want explicit null", key, value, ok)
		}
	}
	if _, ok := decoded["pictureUrl"]; ok {
		t.Error("pictureUrl must be omitted when the channel has no picture")
	}
}

func TestParseNewsletterInviteCodeAcceptsLinksAndCodes(t *testing.T) {
	const code = "0029Va4K0PZ5a245NkngBA2M"
	inputs := []string{
		code,
		"  " + code + "  ",
		"https://whatsapp.com/channel/" + code,
		"https://www.whatsapp.com/channel/" + code,
		"http://whatsapp.com/channel/" + code,
		"whatsapp.com/channel/" + code,
		"www.whatsapp.com/channel/" + code,
		"https://WhatsApp.com/channel/" + code + "/",
		"https://whatsapp.com/channel/" + code + "?utm_source=share",
		"https://whatsapp.com/channel/" + code + "#top",
		"https://whatsapp.com/channel/" + code + "/175",
	}
	for _, input := range inputs {
		got, err := ParseNewsletterInviteCode(input)
		if err != nil || got != code {
			t.Errorf("ParseNewsletterInviteCode(%q) = %q, %v; want %q", input, got, err, code)
		}
	}
}

func TestParseNewsletterInviteCodeRejectsOtherInputs(t *testing.T) {
	inputs := []string{
		"",
		"   ",
		"https://whatsapp.com/channel/",
		"https://chat.whatsapp.com/FXMrTY552nOBFXU71Be8Zh",
		"https://whatsapp.com/group/0029Va4K0PZ5a245NkngBA2M",
		"https://evil.example/channel/0029Va4K0PZ5a245NkngBA2M",
		"https://whatsapp.com.evil.example/channel/0029Va4K0PZ5a245NkngBA2M",
		"ftp://whatsapp.com/channel/0029Va4K0PZ5a245NkngBA2M",
		"120363144038483540@newsletter",
		"code with spaces",
	}
	for _, input := range inputs {
		if got, err := ParseNewsletterInviteCode(input); !errors.Is(err, ErrInvalidNewsletterInvite) {
			t.Errorf("ParseNewsletterInviteCode(%q) = %q, %v; want ErrInvalidNewsletterInvite", input, got, err)
		}
	}
}

func TestFetchAllNewslettersSkipsNilEntries(t *testing.T) {
	client := &fakeNewsletterClient{subscribed: []*types.NewsletterMetadata{
		fullNewsletterMetadata(types.NewsletterRoleAdmin, types.NewsletterMuteOff),
		nil,
	}}
	out, err := fetchAllNewsletters(context.Background(), client)
	if err != nil {
		t.Fatalf("fetchAllNewsletters: %v", err)
	}
	if len(out) != 1 || out[0].Role == nil || *out[0].Role != "admin" {
		t.Fatalf("newsletters = %+v", out)
	}

	empty, err := fetchAllNewsletters(context.Background(), &fakeNewsletterClient{})
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("no channels must encode as an empty list: %v, %v", empty, err)
	}
}

func TestFindNewsletterReportsMissingChannel(t *testing.T) {
	_, err := findNewsletter(context.Background(), &fakeNewsletterClient{}, testNewsletterJID)
	if !errors.Is(err, ErrNewsletterNotFound) {
		t.Fatalf("error = %v, want ErrNewsletterNotFound", err)
	}

	// WhatsApp answers an unknown JID with an empty "non_existing" record.
	nonExisting := &types.NewsletterMetadata{State: types.WrappedNewsletterState{Type: "non_existing"}}
	_, err = findNewsletter(context.Background(), &fakeNewsletterClient{info: nonExisting}, testNewsletterJID)
	if !errors.Is(err, ErrNewsletterNotFound) {
		t.Fatalf("non_existing record: error = %v, want ErrNewsletterNotFound", err)
	}

	wantErr := errors.New("graphql error")
	_, err = findNewsletter(context.Background(), &fakeNewsletterClient{infoErr: wantErr}, testNewsletterJID)
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
}

func TestNewsletterInviteInfoUsesBareCodeAndHasNoViewer(t *testing.T) {
	meta := fullNewsletterMetadata(types.NewsletterRoleOwner, types.NewsletterMuteOff)
	meta.ViewerMeta = nil
	client := &fakeNewsletterClient{info: meta}

	resp, err := newsletterInviteInfo(context.Background(), client, "0029Va4K0PZ5a245NkngBA2M")
	if err != nil {
		t.Fatalf("newsletterInviteInfo: %v", err)
	}
	if client.inviteKey != "0029Va4K0PZ5a245NkngBA2M" {
		t.Fatalf("invite key = %q", client.inviteKey)
	}
	if resp.Role != nil || resp.Muted != nil {
		t.Fatalf("invite lookup must not invent viewer metadata: %+v", resp)
	}

	if _, err := newsletterInviteInfo(context.Background(), &fakeNewsletterClient{}, "abc"); !errors.Is(err, ErrNewsletterNotFound) {
		t.Fatalf("error = %v, want ErrNewsletterNotFound", err)
	}
}

func TestCreateNewsletterReadsBackFullMetadata(t *testing.T) {
	created := &types.NewsletterMetadata{ID: testNewsletterJID, ThreadMeta: types.NewsletterThreadMetadata{Name: types.NewsletterText{Text: "Daily Deals"}}}
	client := &fakeNewsletterClient{
		created: created,
		info:    fullNewsletterMetadata(types.NewsletterRoleOwner, types.NewsletterMuteOff),
	}
	params := whatsmeow.CreateNewsletterParams{Name: "Daily Deals", Description: "Offers", Picture: []byte{1}}

	resp, err := createNewsletter(context.Background(), client, params)
	if err != nil {
		t.Fatalf("createNewsletter: %v", err)
	}
	if client.createParams.Name != "Daily Deals" || client.createParams.Description != "Offers" || len(client.createParams.Picture) != 1 {
		t.Fatalf("create params = %+v", client.createParams)
	}
	if client.infoCallCount != 1 || resp.InviteCode == "" || resp.Muted == nil {
		t.Fatalf("created channel was not read back: calls=%d resp=%+v", client.infoCallCount, resp)
	}
}

func TestCreateNewsletterFallsBackToCreatedMetadataAsOwner(t *testing.T) {
	created := &types.NewsletterMetadata{ID: testNewsletterJID, ThreadMeta: types.NewsletterThreadMetadata{Name: types.NewsletterText{Text: "Daily Deals"}}}
	client := &fakeNewsletterClient{created: created, infoErr: errors.New("temporary failure")}

	resp, err := createNewsletter(context.Background(), client, whatsmeow.CreateNewsletterParams{Name: "Daily Deals"})
	if err != nil {
		t.Fatalf("createNewsletter: %v", err)
	}
	if resp.Id != testNewsletterJID.String() || resp.Role == nil || *resp.Role != "owner" || resp.CanPublish == nil || !*resp.CanPublish {
		t.Fatalf("fallback response = %+v", resp)
	}
	if resp.Muted != nil {
		t.Fatal("mute state is unknown without viewer metadata and must stay null")
	}
}

func TestCreateNewsletterWrapsFailures(t *testing.T) {
	wantErr := errors.New("tos not accepted")
	if _, err := createNewsletter(context.Background(), &fakeNewsletterClient{createErr: wantErr}, whatsmeow.CreateNewsletterParams{Name: "x"}); !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want wrapped %v", err, wantErr)
	}
	if _, err := createNewsletter(context.Background(), &fakeNewsletterClient{}, whatsmeow.CreateNewsletterParams{Name: "x"}); err == nil {
		t.Fatal("empty create response must fail")
	}
}

func TestDecodeNewsletterPictureAcceptsBase64Images(t *testing.T) {
	jpeg := []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00}
	s := &Whatsmiau{httpClient: newMediaHTTPClient()}
	defer s.httpClient.CloseIdleConnections()

	for _, input := range []string{
		base64.StdEncoding.EncodeToString(jpeg),
		"data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(jpeg),
	} {
		picture, err := s.decodeNewsletterPicture(context.Background(), input)
		if err != nil || len(picture) != len(jpeg) {
			t.Fatalf("decodeNewsletterPicture(%q) = %d bytes, %v", input[:12], len(picture), err)
		}
	}
}

func TestDecodeNewsletterPictureRejectsNonImagesAndPrivateURLs(t *testing.T) {
	var hits atomic.Int32
	private := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
	}))
	defer private.Close()

	s := &Whatsmiau{httpClient: newMediaHTTPClient()}
	defer s.httpClient.CloseIdleConnections()

	for _, input := range []string{
		base64.StdEncoding.EncodeToString([]byte("plain text, not an image")),
		"not base64 at all!",
		private.URL + "/picture.jpg",
	} {
		if _, err := s.decodeNewsletterPicture(context.Background(), input); !errors.Is(err, ErrInvalidNewsletterPicture) {
			t.Errorf("decodeNewsletterPicture(%q) error = %v, want ErrInvalidNewsletterPicture", input, err)
		}
	}
	if hits.Load() != 0 {
		t.Fatal("picture download reached a loopback address")
	}
}

func TestNewsletterOperationsRequireAConnectedInstance(t *testing.T) {
	s := &Whatsmiau{clients: xsync.NewMap[string, *whatsmeow.Client]()}
	ctx := context.Background()
	req := &NewsletterRequest{InstanceID: "missing", NewsletterJID: &testNewsletterJID}

	if _, err := s.FetchAllNewsletters(ctx, "missing"); !errors.Is(err, whatsmeow.ErrClientIsNil) {
		t.Errorf("FetchAllNewsletters error = %v", err)
	}
	if _, err := s.FindNewsletter(ctx, req); !errors.Is(err, whatsmeow.ErrClientIsNil) {
		t.Errorf("FindNewsletter error = %v", err)
	}
	if _, err := s.CreateNewsletter(ctx, &CreateNewsletterRequest{InstanceID: "missing", Name: "x"}); !errors.Is(err, whatsmeow.ErrClientIsNil) {
		t.Errorf("CreateNewsletter error = %v", err)
	}
	if err := s.FollowNewsletter(ctx, req); !errors.Is(err, whatsmeow.ErrClientIsNil) {
		t.Errorf("FollowNewsletter error = %v", err)
	}
	if err := s.SetNewsletterMute(ctx, req, true); !errors.Is(err, whatsmeow.ErrClientIsNil) {
		t.Errorf("SetNewsletterMute error = %v", err)
	}
}

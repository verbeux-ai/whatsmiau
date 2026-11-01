package whatsmiau

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
	"go.uber.org/zap"
)

var (
	// ErrNewsletterNotFound is returned when WhatsApp answers a channel lookup
	// without metadata, which is how unknown channels are reported.
	ErrNewsletterNotFound = errors.New("newsletter not found")
	// ErrInvalidNewsletterInvite is returned for invite inputs that are neither
	// a channel invite code nor a whatsapp.com/channel link.
	ErrInvalidNewsletterInvite = errors.New("invalid newsletter invite")
	// ErrInvalidNewsletterPicture is returned when the channel picture cannot be
	// downloaded, decoded or is not an image.
	ErrInvalidNewsletterPicture = errors.New("invalid newsletter picture")
)

var newsletterInviteCodePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

// ---------- Shared response shape ----------

// NewsletterResponse describes a WhatsApp channel (newsletter). Role, Muted and
// CanPublish are null when WhatsApp did not send viewer metadata, which is the
// case for lookups by invite code.
type NewsletterResponse struct {
	Id                string  `json:"id" example:"120363144038483540@newsletter"`
	Name              string  `json:"name"`
	Description       string  `json:"description"`
	InviteCode        string  `json:"inviteCode,omitempty" example:"0029Va4K0PZ5a245NkngBA2M"`
	InviteURL         string  `json:"inviteUrl,omitempty" example:"https://whatsapp.com/channel/0029Va4K0PZ5a245NkngBA2M"`
	PictureID         string  `json:"pictureId,omitempty"`
	PictureURL        string  `json:"pictureUrl,omitempty"`
	PictureDirectPath string  `json:"pictureDirectPath,omitempty"`
	PreviewURL        string  `json:"previewUrl,omitempty"`
	SubscribersCount  int     `json:"subscribersCount"`
	Verified          bool    `json:"verified"`
	State             string  `json:"state,omitempty" enums:"active,suspended,geosuspended"`
	Creation          int64   `json:"creation,omitempty"`
	ReactionsMode     string  `json:"reactionsMode,omitempty" enums:"all,basic,none,blocklist"`
	Role              *string `json:"role" enums:"owner,admin,subscriber,guest" extensions:"x-nullable"`
	Muted             *bool   `json:"muted" extensions:"x-nullable"`
	CanPublish        *bool   `json:"canPublish" extensions:"x-nullable"`
}

func buildNewsletterResponse(meta *types.NewsletterMetadata) NewsletterResponse {
	thread := meta.ThreadMeta
	resp := NewsletterResponse{
		Id:               meta.ID.String(),
		Name:             thread.Name.Text,
		Description:      thread.Description.Text,
		InviteCode:       thread.InviteCode,
		PreviewURL:       thread.Preview.URL,
		SubscribersCount: thread.SubscriberCount,
		Verified:         thread.VerificationState == types.NewsletterVerificationStateVerified,
		State:            string(meta.State.Type),
		ReactionsMode:    string(thread.Settings.ReactionCodes.Value),
	}
	if thread.InviteCode != "" {
		resp.InviteURL = whatsmeow.NewsletterLinkPrefix + thread.InviteCode
	}
	if thread.Picture != nil {
		resp.PictureID = thread.Picture.ID
		resp.PictureURL = thread.Picture.URL
		resp.PictureDirectPath = thread.Picture.DirectPath
	}
	if !thread.CreationTime.IsZero() {
		resp.Creation = thread.CreationTime.Unix()
	}
	if meta.ViewerMeta != nil {
		setNewsletterViewer(&resp, meta.ViewerMeta.Role, meta.ViewerMeta.Mute == types.NewsletterMuteOn, true)
	}
	return resp
}

// setNewsletterViewer fills the viewer-dependent fields. mutedKnown=false keeps
// Muted null when only the role can be inferred.
func setNewsletterViewer(resp *NewsletterResponse, role types.NewsletterRole, muted, mutedKnown bool) {
	if role != "" {
		value := string(role)
		canPublish := role == types.NewsletterRoleOwner || role == types.NewsletterRoleAdmin
		resp.Role = &value
		resp.CanPublish = &canPublish
	}
	if mutedKnown {
		resp.Muted = &muted
	}
}

// ParseNewsletterInviteCode extracts the invite code from a channel link
// (https://whatsapp.com/channel/CODE, with or without www., scheme, trailing
// path such as a post ID, or query string) or validates a bare code.
func ParseNewsletterInviteCode(input string) (string, error) {
	code := strings.TrimSpace(input)
	if code == "" {
		return "", fmt.Errorf("%w: invite code is required", ErrInvalidNewsletterInvite)
	}

	if strings.ContainsAny(code, "/:?#") {
		raw := code
		if !strings.Contains(raw, "://") {
			raw = "https://" + raw
		}
		u, err := url.Parse(raw)
		if err != nil {
			return "", fmt.Errorf("%w: %v", ErrInvalidNewsletterInvite, err)
		}
		host := strings.ToLower(u.Hostname())
		if (u.Scheme != "https" && u.Scheme != "http") || (host != "whatsapp.com" && host != "www.whatsapp.com") {
			return "", fmt.Errorf("%w: expected a https://whatsapp.com/channel/ link", ErrInvalidNewsletterInvite)
		}
		rest, ok := strings.CutPrefix(strings.Trim(u.Path, "/"), "channel/")
		if !ok {
			return "", fmt.Errorf("%w: expected a https://whatsapp.com/channel/ link", ErrInvalidNewsletterInvite)
		}
		// Post links (channel/CODE/123) still identify the channel by CODE.
		code, _, _ = strings.Cut(rest, "/")
	}

	if !newsletterInviteCodePattern.MatchString(code) {
		return "", fmt.Errorf("%w: malformed invite code", ErrInvalidNewsletterInvite)
	}
	return code, nil
}

// newsletterClient is the subset of *whatsmeow.Client used for channel
// management, kept as an interface so the logic is testable without WhatsApp.
type newsletterClient interface {
	GetSubscribedNewsletters(ctx context.Context) ([]*types.NewsletterMetadata, error)
	GetNewsletterInfo(ctx context.Context, jid types.JID) (*types.NewsletterMetadata, error)
	GetNewsletterInfoWithInvite(ctx context.Context, key string) (*types.NewsletterMetadata, error)
	CreateNewsletter(ctx context.Context, params whatsmeow.CreateNewsletterParams) (*types.NewsletterMetadata, error)
	FollowNewsletter(ctx context.Context, jid types.JID) error
	UnfollowNewsletter(ctx context.Context, jid types.JID) error
	NewsletterToggleMute(ctx context.Context, jid types.JID, mute bool) error
}

func (s *Whatsmiau) loadNewsletterClient(instanceID string) (newsletterClient, error) {
	client, ok := s.clients.Load(instanceID)
	if !ok || client == nil {
		return nil, whatsmeow.ErrClientIsNil
	}
	return client, nil
}

// ---------- FetchAllNewsletters ----------

func (s *Whatsmiau) FetchAllNewsletters(ctx context.Context, instanceID string) ([]NewsletterResponse, error) {
	client, err := s.loadNewsletterClient(instanceID)
	if err != nil {
		return nil, err
	}
	return fetchAllNewsletters(ctx, client)
}

func fetchAllNewsletters(ctx context.Context, client newsletterClient) ([]NewsletterResponse, error) {
	newsletters, err := client.GetSubscribedNewsletters(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]NewsletterResponse, 0, len(newsletters))
	for _, meta := range newsletters {
		if meta == nil {
			continue
		}
		out = append(out, buildNewsletterResponse(meta))
	}
	return out, nil
}

// ---------- FindNewsletter / NewsletterInviteInfo ----------

type NewsletterRequest struct {
	InstanceID    string
	NewsletterJID *types.JID
}

func (s *Whatsmiau) FindNewsletter(ctx context.Context, req *NewsletterRequest) (*NewsletterResponse, error) {
	client, err := s.loadNewsletterClient(req.InstanceID)
	if err != nil {
		return nil, err
	}
	return findNewsletter(ctx, client, *req.NewsletterJID)
}

func findNewsletter(ctx context.Context, client newsletterClient, jid types.JID) (*NewsletterResponse, error) {
	meta, err := client.GetNewsletterInfo(ctx, jid)
	if err != nil {
		return nil, err
	}
	if meta == nil {
		return nil, fmt.Errorf("%w: %s", ErrNewsletterNotFound, jid)
	}
	resp := buildNewsletterResponse(meta)
	return &resp, nil
}

type NewsletterInviteRequest struct {
	InstanceID string
	// InviteCode is the bare code, already extracted by ParseNewsletterInviteCode.
	InviteCode string
}

func (s *Whatsmiau) NewsletterInviteInfo(ctx context.Context, req *NewsletterInviteRequest) (*NewsletterResponse, error) {
	client, err := s.loadNewsletterClient(req.InstanceID)
	if err != nil {
		return nil, err
	}
	return newsletterInviteInfo(ctx, client, req.InviteCode)
}

func newsletterInviteInfo(ctx context.Context, client newsletterClient, code string) (*NewsletterResponse, error) {
	meta, err := client.GetNewsletterInfoWithInvite(ctx, code)
	if err != nil {
		return nil, err
	}
	if meta == nil {
		return nil, fmt.Errorf("%w: invite %s", ErrNewsletterNotFound, code)
	}
	resp := buildNewsletterResponse(meta)
	return &resp, nil
}

// ---------- CreateNewsletter ----------

type CreateNewsletterRequest struct {
	InstanceID  string
	Name        string
	Description string
	// Picture is an http(s) URL, a data: URI or raw base64 of an image (JPEG recommended).
	Picture string
}

func (s *Whatsmiau) CreateNewsletter(ctx context.Context, req *CreateNewsletterRequest) (*NewsletterResponse, error) {
	client, err := s.loadNewsletterClient(req.InstanceID)
	if err != nil {
		return nil, err
	}

	params := whatsmeow.CreateNewsletterParams{
		Name:        req.Name,
		Description: req.Description,
	}
	if req.Picture != "" {
		if params.Picture, err = s.decodeNewsletterPicture(ctx, req.Picture); err != nil {
			return nil, err
		}
	}
	return createNewsletter(ctx, client, params)
}

// decodeNewsletterPicture reuses the guarded media download used for group
// pictures and rejects payloads that are not images.
func (s *Whatsmiau) decodeNewsletterPicture(ctx context.Context, input string) ([]byte, error) {
	picture, err := s.decodeImageInput(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidNewsletterPicture, err)
	}
	if len(picture) == 0 {
		return nil, fmt.Errorf("%w: empty image", ErrInvalidNewsletterPicture)
	}
	if contentType := http.DetectContentType(picture); !strings.HasPrefix(contentType, "image/") {
		return nil, fmt.Errorf("%w: expected an image, got %s", ErrInvalidNewsletterPicture, contentType)
	}
	return picture, nil
}

func createNewsletter(ctx context.Context, client newsletterClient, params whatsmeow.CreateNewsletterParams) (*NewsletterResponse, error) {
	created, err := client.CreateNewsletter(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to create newsletter: %w", err)
	}
	if created == nil {
		return nil, fmt.Errorf("failed to create newsletter: empty response")
	}

	meta := created
	if !created.ID.IsEmpty() {
		// The creation response may omit viewer metadata, so read it back.
		info, err := client.GetNewsletterInfo(ctx, created.ID)
		if err != nil {
			zap.L().Warn("newsletter created but failed to fetch its info", zap.Error(err), zap.String("newsletter", created.ID.String()))
		} else if info != nil {
			meta = info
		}
	}

	resp := buildNewsletterResponse(meta)
	if resp.Role == nil {
		// The creator owns the channel even when WhatsApp omits viewer metadata.
		setNewsletterViewer(&resp, types.NewsletterRoleOwner, false, false)
	}
	return &resp, nil
}

// ---------- Follow / Unfollow / Mute ----------

func (s *Whatsmiau) FollowNewsletter(ctx context.Context, req *NewsletterRequest) error {
	client, err := s.loadNewsletterClient(req.InstanceID)
	if err != nil {
		return err
	}
	return client.FollowNewsletter(ctx, *req.NewsletterJID)
}

func (s *Whatsmiau) UnfollowNewsletter(ctx context.Context, req *NewsletterRequest) error {
	client, err := s.loadNewsletterClient(req.InstanceID)
	if err != nil {
		return err
	}
	return client.UnfollowNewsletter(ctx, *req.NewsletterJID)
}

func (s *Whatsmiau) SetNewsletterMute(ctx context.Context, req *NewsletterRequest, mute bool) error {
	client, err := s.loadNewsletterClient(req.InstanceID)
	if err != nil {
		return err
	}
	return client.NewsletterToggleMute(ctx, *req.NewsletterJID, mute)
}

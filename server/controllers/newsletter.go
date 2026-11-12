package controllers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/verbeux-ai/whatsmiau/lib/whatsmiau"
	"github.com/verbeux-ai/whatsmiau/server/dto"
	"github.com/verbeux-ai/whatsmiau/utils"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
	"go.uber.org/zap"
)

type Newsletter struct {
	whatsmiau *whatsmiau.Whatsmiau
	validate  *validator.Validate
}

func NewNewsletters(w *whatsmiau.Whatsmiau) *Newsletter {
	return &Newsletter{whatsmiau: w, validate: validator.New()}
}

// parseNewsletterJID accepts "<id>@newsletter" or the bare numeric id and
// rejects any other server, so group or user JIDs fail with a clear 400.
func parseNewsletterJID(input string) (*types.JID, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, fmt.Errorf("newsletter jid is required")
	}
	if !strings.Contains(input, "@") {
		input += "@" + types.NewsletterServer
	}

	jid, err := types.ParseJID(input)
	if err != nil {
		return nil, fmt.Errorf("invalid newsletter jid: %w", err)
	}
	if jid.Server != types.NewsletterServer {
		return nil, fmt.Errorf("not a newsletter jid: %s", jid.String())
	}
	if jid.User == "" || strings.Trim(jid.User, "0123456789") != "" || jid.Device != 0 {
		return nil, fmt.Errorf("invalid newsletter jid: %s", input)
	}
	return &jid, nil
}

func mapNewsletterError(err error) (int, string) {
	var gqlErr types.GraphQLError
	switch {
	case errors.Is(err, whatsmeow.ErrClientIsNil), errors.Is(err, whatsmeow.ErrNotConnected), errors.Is(err, whatsmeow.ErrNotLoggedIn):
		return http.StatusNotFound, "instance is not connected"
	case errors.Is(err, whatsmiau.ErrNewsletterNotFound), errors.Is(err, whatsmeow.ErrIQNotFound):
		return http.StatusNotFound, "newsletter not found"
	case errors.Is(err, whatsmiau.ErrInvalidNewsletterPicture):
		return http.StatusBadRequest, "invalid picture"
	case errors.Is(err, whatsmeow.ErrIQBadRequest):
		return http.StatusBadRequest, "WhatsApp rejected the newsletter operation"
	case errors.Is(err, whatsmeow.ErrIQNotAuthorized), errors.Is(err, whatsmeow.ErrIQForbidden):
		return http.StatusForbidden, "WhatsApp rejected the newsletter permissions"
	case errors.Is(err, whatsmeow.ErrIQRateOverLimit):
		return http.StatusTooManyRequests, "rate limit exceeded, try again later"
	case errors.As(err, &gqlErr):
		// Channel operations go through WhatsApp's GraphQL endpoint, which
		// reports failures with HTTP-like codes instead of IQ errors.
		switch code := gqlErr.Extensions.ErrorCode; {
		case code == http.StatusNotFound:
			return http.StatusNotFound, "newsletter not found"
		case code == http.StatusBadRequest:
			return http.StatusBadRequest, "WhatsApp rejected the newsletter operation"
		case code == http.StatusUnauthorized, code == http.StatusForbidden:
			return http.StatusForbidden, "WhatsApp rejected the newsletter permissions"
		case code == http.StatusTooManyRequests:
			return http.StatusTooManyRequests, "rate limit exceeded, try again later"
		}
	}
	return http.StatusInternalServerError, "operation failed"
}

func newsletterFail(ctx echo.Context, operation string, err error) error {
	zap.L().Error("Whatsmiau."+operation+" failed", zap.Error(err))
	code, msg := mapNewsletterError(err)
	return utils.HTTPFail(ctx, code, err, msg)
}

// FetchAllNewsletters godoc
// @Summary      List the instance's channels (newsletters)
// @Description  Returns every WhatsApp channel the number follows or administers. role is owner, admin, subscriber or guest; canPublish is true for owner and admin.
// @Tags         Newsletter
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance  path      string  true  "Instance ID"
// @Success      200       {array}   whatsmiau.NewsletterResponse
// @Failure      400       {object}  utils.HTTPErrorResponse
// @Failure      403       {object}  utils.HTTPErrorResponse
// @Failure      404       {object}  utils.HTTPErrorResponse
// @Failure      429       {object}  utils.HTTPErrorResponse
// @Failure      500       {object}  utils.HTTPErrorResponse
// @Router       /v1/instance/{instance}/newsletter/fetchAll [get]
func (s *Newsletter) FetchAll(ctx echo.Context) error {
	var request dto.NewsletterInstanceRequest
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request")
	}
	if err := s.validate.Struct(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request")
	}

	resp, err := s.whatsmiau.FetchAllNewsletters(ctx.Request().Context(), request.InstanceID)
	if err != nil {
		return newsletterFail(ctx, "FetchAllNewsletters", err)
	}
	return ctx.JSON(http.StatusOK, resp)
}

// Info godoc
// @Summary      Get a channel (newsletter) by JID
// @Tags         Newsletter
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance       path      string  true  "Instance ID"
// @Param        newsletterJid  query     string  true  "Channel JID (<id>@newsletter or the numeric id)"
// @Success      200            {object}  whatsmiau.NewsletterResponse
// @Failure      400            {object}  utils.HTTPErrorResponse
// @Failure      403            {object}  utils.HTTPErrorResponse
// @Failure      404            {object}  utils.HTTPErrorResponse
// @Failure      422            {object}  utils.HTTPErrorResponse
// @Failure      429            {object}  utils.HTTPErrorResponse
// @Failure      500            {object}  utils.HTTPErrorResponse
// @Router       /v1/instance/{instance}/newsletter/info [get]
func (s *Newsletter) Info(ctx echo.Context) error {
	var request dto.NewsletterJidQuery
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request")
	}
	if err := s.validate.Struct(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request")
	}
	jid, err := parseNewsletterJID(request.NewsletterJid)
	if err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid newsletterJid")
	}

	resp, err := s.whatsmiau.FindNewsletter(ctx.Request().Context(), &whatsmiau.NewsletterRequest{
		InstanceID:    request.InstanceID,
		NewsletterJID: jid,
	})
	if err != nil {
		return newsletterFail(ctx, "FindNewsletter", err)
	}
	return ctx.JSON(http.StatusOK, resp)
}

// InviteInfo godoc
// @Summary      Get a channel (newsletter) by invite link or code
// @Description  Accepts https://whatsapp.com/channel/CODE, https://www.whatsapp.com/channel/CODE (post links included) or the bare CODE. WhatsApp does not return viewer metadata for invite lookups, so role, muted and canPublish are null.
// @Tags         Newsletter
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance    path      string  true  "Instance ID"
// @Param        inviteCode  query     string  true  "Channel invite link or code"
// @Success      200         {object}  whatsmiau.NewsletterResponse
// @Failure      400         {object}  utils.HTTPErrorResponse
// @Failure      403         {object}  utils.HTTPErrorResponse
// @Failure      404         {object}  utils.HTTPErrorResponse
// @Failure      422         {object}  utils.HTTPErrorResponse
// @Failure      429         {object}  utils.HTTPErrorResponse
// @Failure      500         {object}  utils.HTTPErrorResponse
// @Router       /v1/instance/{instance}/newsletter/inviteInfo [get]
func (s *Newsletter) InviteInfo(ctx echo.Context) error {
	var request dto.NewsletterInviteQuery
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request")
	}
	if err := s.validate.Struct(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request")
	}
	code, err := whatsmiau.ParseNewsletterInviteCode(request.InviteCode)
	if err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid inviteCode")
	}

	resp, err := s.whatsmiau.NewsletterInviteInfo(ctx.Request().Context(), &whatsmiau.NewsletterInviteRequest{
		InstanceID: request.InstanceID,
		InviteCode: code,
	})
	if err != nil {
		return newsletterFail(ctx, "NewsletterInviteInfo", err)
	}
	return ctx.JSON(http.StatusOK, resp)
}

// Create godoc
// @Summary      Create a channel (newsletter)
// @Description  Creates a WhatsApp channel owned by the instance. picture accepts an http(s) URL (public addresses only), a data: URI or raw base64; JPEG is recommended.
// @Tags         Newsletter
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance  path      string                       true  "Instance ID"
// @Param        body      body      dto.CreateNewsletterRequest  true  "Channel payload"
// @Success      201       {object}  whatsmiau.NewsletterResponse
// @Failure      400       {object}  utils.HTTPErrorResponse
// @Failure      403       {object}  utils.HTTPErrorResponse
// @Failure      404       {object}  utils.HTTPErrorResponse
// @Failure      422       {object}  utils.HTTPErrorResponse
// @Failure      429       {object}  utils.HTTPErrorResponse
// @Failure      500       {object}  utils.HTTPErrorResponse
// @Router       /v1/instance/{instance}/newsletter/create [post]
func (s *Newsletter) Create(ctx echo.Context) error {
	var request dto.CreateNewsletterRequest
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request body")
	}
	if err := s.validate.Struct(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request body")
	}
	name := strings.TrimSpace(request.Name)
	if name == "" {
		return utils.HTTPFail(ctx, http.StatusBadRequest, fmt.Errorf("name is blank"), "invalid request body")
	}

	resp, err := s.whatsmiau.CreateNewsletter(ctx.Request().Context(), &whatsmiau.CreateNewsletterRequest{
		InstanceID:  request.InstanceID,
		Name:        name,
		Description: request.Description,
		Picture:     strings.TrimSpace(request.Picture),
	})
	if err != nil {
		return newsletterFail(ctx, "CreateNewsletter", err)
	}
	return ctx.JSON(http.StatusCreated, resp)
}

// Follow godoc
// @Summary      Follow a channel (newsletter)
// @Tags         Newsletter
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance  path      string                    true  "Instance ID"
// @Param        body      body      dto.NewsletterJidRequest  true  "Channel JID"
// @Success      200       {object}  dto.NewsletterActionResponse
// @Failure      400       {object}  utils.HTTPErrorResponse
// @Failure      403       {object}  utils.HTTPErrorResponse
// @Failure      404       {object}  utils.HTTPErrorResponse
// @Failure      422       {object}  utils.HTTPErrorResponse
// @Failure      429       {object}  utils.HTTPErrorResponse
// @Failure      500       {object}  utils.HTTPErrorResponse
// @Router       /v1/instance/{instance}/newsletter/follow [post]
func (s *Newsletter) Follow(ctx echo.Context) error {
	return s.action(ctx, "follow", "FollowNewsletter", s.whatsmiau.FollowNewsletter)
}

// Unfollow godoc
// @Summary      Unfollow a channel (newsletter)
// @Tags         Newsletter
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance  path      string                    true  "Instance ID"
// @Param        body      body      dto.NewsletterJidRequest  true  "Channel JID"
// @Success      200       {object}  dto.NewsletterActionResponse
// @Failure      400       {object}  utils.HTTPErrorResponse
// @Failure      403       {object}  utils.HTTPErrorResponse
// @Failure      404       {object}  utils.HTTPErrorResponse
// @Failure      422       {object}  utils.HTTPErrorResponse
// @Failure      429       {object}  utils.HTTPErrorResponse
// @Failure      500       {object}  utils.HTTPErrorResponse
// @Router       /v1/instance/{instance}/newsletter/unfollow [post]
func (s *Newsletter) Unfollow(ctx echo.Context) error {
	return s.action(ctx, "unfollow", "UnfollowNewsletter", s.whatsmiau.UnfollowNewsletter)
}

// Mute godoc
// @Summary      Mute a channel (newsletter)
// @Tags         Newsletter
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance  path      string                    true  "Instance ID"
// @Param        body      body      dto.NewsletterJidRequest  true  "Channel JID"
// @Success      200       {object}  dto.NewsletterActionResponse
// @Failure      400       {object}  utils.HTTPErrorResponse
// @Failure      403       {object}  utils.HTTPErrorResponse
// @Failure      404       {object}  utils.HTTPErrorResponse
// @Failure      422       {object}  utils.HTTPErrorResponse
// @Failure      429       {object}  utils.HTTPErrorResponse
// @Failure      500       {object}  utils.HTTPErrorResponse
// @Router       /v1/instance/{instance}/newsletter/mute [post]
func (s *Newsletter) Mute(ctx echo.Context) error {
	return s.action(ctx, "mute", "SetNewsletterMute", func(c context.Context, req *whatsmiau.NewsletterRequest) error {
		return s.whatsmiau.SetNewsletterMute(c, req, true)
	})
}

// Unmute godoc
// @Summary      Unmute a channel (newsletter)
// @Tags         Newsletter
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance  path      string                    true  "Instance ID"
// @Param        body      body      dto.NewsletterJidRequest  true  "Channel JID"
// @Success      200       {object}  dto.NewsletterActionResponse
// @Failure      400       {object}  utils.HTTPErrorResponse
// @Failure      403       {object}  utils.HTTPErrorResponse
// @Failure      404       {object}  utils.HTTPErrorResponse
// @Failure      422       {object}  utils.HTTPErrorResponse
// @Failure      429       {object}  utils.HTTPErrorResponse
// @Failure      500       {object}  utils.HTTPErrorResponse
// @Router       /v1/instance/{instance}/newsletter/unmute [post]
func (s *Newsletter) Unmute(ctx echo.Context) error {
	return s.action(ctx, "unmute", "SetNewsletterMute", func(c context.Context, req *whatsmiau.NewsletterRequest) error {
		return s.whatsmiau.SetNewsletterMute(c, req, false)
	})
}

func (s *Newsletter) action(ctx echo.Context, action, operation string, run func(context.Context, *whatsmiau.NewsletterRequest) error) error {
	var request dto.NewsletterJidRequest
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request body")
	}
	if err := s.validate.Struct(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request body")
	}
	jid, err := parseNewsletterJID(request.NewsletterJid)
	if err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid newsletterJid")
	}

	if err := run(ctx.Request().Context(), &whatsmiau.NewsletterRequest{
		InstanceID:    request.InstanceID,
		NewsletterJID: jid,
	}); err != nil {
		return newsletterFail(ctx, operation, err)
	}
	return ctx.JSON(http.StatusOK, dto.NewsletterActionResponse{
		NewsletterJid: jid.String(),
		Action:        action,
	})
}

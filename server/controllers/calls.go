package controllers

import (
	"context"
	"encoding/binary"
	"math"
	"net/http"
	"strings"
	"sync/atomic"

	"github.com/coder/websocket"
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/verbeux-ai/whatsmiau/lib/whatsmiau"
	"github.com/verbeux-ai/whatsmiau/server/dto"
	"github.com/verbeux-ai/whatsmiau/utils"
	"go.uber.org/zap"
)

const (
	callAudioSamples = 960
	callAudioBytes   = callAudioSamples * 4
)

type Calls struct {
	whatsmiau *whatsmiau.Whatsmiau
	validate  *validator.Validate
}

func NewCalls(w *whatsmiau.Whatsmiau) *Calls {
	return &Calls{whatsmiau: w, validate: validator.New()}
}

// Offer places a direct 1:1 audio call.
// @Summary Place an audio call
// @Tags Calls
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param instance path string true "Instance ID"
// @Param body body dto.CallOfferRequest true "Call target"
// @Success 201 {object} dto.CallOfferResponse
// @Failure 400 {object} utils.HTTPErrorResponse
// @Failure 401 {object} utils.AuthenticationErrorResponse
// @Failure 422 {object} utils.HTTPErrorResponse
// @Failure 409 {object} utils.HTTPErrorResponse
// @Router /v1/instance/{instance}/calls [post]
func (s *Calls) Offer(ctx echo.Context) error {
	var request dto.CallOfferRequest
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request body")
	}
	if err := s.validate.Struct(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request body")
	}
	jid, err := numberToJid(strings.TrimSpace(request.Number))
	if err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid call target")
	}
	offer, err := s.whatsmiau.OfferAudioCall(ctx.Request().Context(), ctx.Param("instance"), jid)
	if err != nil {
		return utils.HTTPFail(ctx, http.StatusConflict, err, "failed to place call")
	}
	return ctx.JSON(http.StatusCreated, offer)
}

// List returns safe lifecycle views for calls owned by an instance.
// @Summary List calls
// @Tags Calls
// @Produce json
// @Security ApiKeyAuth
// @Param instance path string true "Instance ID"
// @Success 200 {array} dto.CallSessionResponse
// @Failure 401 {object} utils.AuthenticationErrorResponse
// @Failure 404 {object} utils.HTTPErrorResponse
// @Router /v1/instance/{instance}/calls [get]
func (s *Calls) List(ctx echo.Context) error {
	calls, err := s.whatsmiau.ListCallSessions(ctx.Param("instance"))
	if err != nil {
		return utils.HTTPFail(ctx, http.StatusNotFound, err, "call support or instance not found")
	}
	return ctx.JSON(http.StatusOK, calls)
}

// Answer accepts a ringing incoming direct call.
// @Summary Answer an incoming call
// @Description Answers only a currently ringing incoming call. The call session capability flags indicate whether this action is allowed.
// @Tags Calls
// @Produce json
// @Security ApiKeyAuth
// @Param instance path string true "Instance ID"
// @Param callID path string true "Call ID"
// @Success 200 {object} dto.CallActionResponse
// @Failure 401 {object} utils.AuthenticationErrorResponse
// @Failure 409 {object} utils.HTTPErrorResponse
// @Router /v1/instance/{instance}/calls/{callID}/answer [post]
func (s *Calls) Answer(ctx echo.Context) error { return s.control(ctx, "answer") }

// Reject declines a ringing incoming direct call.
// @Summary Reject an incoming call
// @Tags Calls
// @Produce json
// @Security ApiKeyAuth
// @Param instance path string true "Instance ID"
// @Param callID path string true "Call ID"
// @Success 200 {object} dto.CallActionResponse
// @Failure 401 {object} utils.AuthenticationErrorResponse
// @Failure 409 {object} utils.HTTPErrorResponse
// @Router /v1/instance/{instance}/calls/{callID}/reject [post]
func (s *Calls) Reject(ctx echo.Context) error { return s.control(ctx, "reject") }

// Hangup terminates an active, connecting, ringing, or outgoing call.
// @Summary Hang up a call
// @Tags Calls
// @Produce json
// @Security ApiKeyAuth
// @Param instance path string true "Instance ID"
// @Param callID path string true "Call ID"
// @Success 200 {object} dto.CallActionResponse
// @Failure 401 {object} utils.AuthenticationErrorResponse
// @Failure 409 {object} utils.HTTPErrorResponse
// @Router /v1/instance/{instance}/calls/{callID}/hangup [post]
func (s *Calls) Hangup(ctx echo.Context) error { return s.control(ctx, "hangup") }

func (s *Calls) control(ctx echo.Context, action string) error {
	var err error
	switch action {
	case "answer":
		err = s.whatsmiau.AnswerIncomingCall(ctx.Param("instance"), ctx.Param("callID"))
	case "reject":
		err = s.whatsmiau.RejectIncomingCall(ctx.Request().Context(), ctx.Param("instance"), ctx.Param("callID"))
	case "hangup":
		err = s.whatsmiau.HangupCall(ctx.Param("instance"), ctx.Param("callID"))
	}
	if err != nil {
		return utils.HTTPFail(ctx, http.StatusConflict, err, "call action failed")
	}
	return ctx.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// Audio upgrades to a binary audio bridge. The WebSocket handshake is already
// protected by the /v1 apikey middleware. Frames are float32 PCM, mono 16 kHz,
// 960 samples each; no WhatsApp relay data or keys cross this boundary.
// @Summary Open the call audio WebSocket
// @Description Upgrades to WebSocket. Each client-to-call and call-to-client binary message must be exactly 3,840 bytes: 960 little-endian float32 PCM samples, mono, 16 kHz (60 ms). No SDP, relay data, encrypted media, or call keys cross this boundary.
// @Tags Calls
// @Produce application/octet-stream
// @Security ApiKeyAuth
// @Param instance path string true "Instance ID"
// @Param callID path string true "Call ID"
// @Success 101 {string} string "WebSocket Switching Protocols"
// @Failure 401 {object} utils.AuthenticationErrorResponse
// @Failure 404 {object} utils.HTTPErrorResponse
// @Router /v1/instance/{instance}/calls/{callID}/audio [get]
func (s *Calls) Audio(ctx echo.Context) error {
	stream, err := s.whatsmiau.OpenCallAudio(ctx.Param("instance"), ctx.Param("callID"))
	if err != nil {
		return utils.HTTPFail(ctx, http.StatusNotFound, err, "call session not found")
	}
	defer stream.Close()

	conn, err := websocket.Accept(ctx.Response(), ctx.Request(), &websocket.AcceptOptions{CompressionMode: websocket.CompressionDisabled})
	if err != nil {
		return nil
	}
	defer conn.Close(websocket.StatusNormalClosure, "audio bridge closed")
	conn.SetReadLimit(callAudioBytes)

	bridgeContext, cancel := context.WithCancel(ctx.Request().Context())
	defer cancel()
	var fromClient, toClient atomic.Uint64
	defer func() {
		zap.L().Info("call API audio bridge closed", zap.String("instance", ctx.Param("instance")), zap.String("call_id", ctx.Param("callID")), zap.Uint64("client_to_call_frames", fromClient.Load()), zap.Uint64("call_to_client_frames", toClient.Load()))
	}()
	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		for frame := range stream.Receive {
			if err := conn.Write(bridgeContext, websocket.MessageBinary, encodeCallPCM(frame)); err != nil {
				cancel()
				return
			}
			toClient.Add(1)
		}
	}()
	for {
		messageType, payload, err := conn.Read(bridgeContext)
		if err != nil {
			break
		}
		frame, valid := decodeCallPCM(payload)
		if messageType != websocket.MessageBinary || !valid || stream.Push(frame) != nil {
			_ = conn.Close(websocket.StatusPolicyViolation, "expected one valid PCM frame")
			break
		}
		fromClient.Add(1)
	}
	cancel()
	<-writerDone
	return nil
}

func encodeCallPCM(frame []float32) []byte {
	payload := make([]byte, callAudioBytes)
	for i := 0; i < len(frame) && i < callAudioSamples; i++ {
		binary.LittleEndian.PutUint32(payload[i*4:], math.Float32bits(frame[i]))
	}
	return payload
}

func decodeCallPCM(payload []byte) ([]float32, bool) {
	if len(payload) != callAudioBytes {
		return nil, false
	}
	frame := make([]float32, callAudioSamples)
	for i := range frame {
		sample := math.Float32frombits(binary.LittleEndian.Uint32(payload[i*4:]))
		if math.IsNaN(float64(sample)) || math.IsInf(float64(sample), 0) {
			return nil, false
		}
		frame[i] = max(-1, min(1, sample))
	}
	return frame, true
}

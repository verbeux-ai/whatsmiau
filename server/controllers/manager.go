package controllers

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"html/template"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	qrcode "github.com/skip2/go-qrcode"
	"github.com/verbeux-ai/whatsmiau/env"
	"github.com/verbeux-ai/whatsmiau/interfaces"
	"github.com/verbeux-ai/whatsmiau/lib/whatsmiau"
	"github.com/verbeux-ai/whatsmiau/models"
	"github.com/verbeux-ai/whatsmiau/repositories/instances"
	"github.com/verbeux-ai/whatsmiau/server/dto"
	"github.com/verbeux-ai/whatsmiau/server/middleware"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

var webhookEventOptions = map[whatsmiau.Wook]bool{
	whatsmiau.WookMessagesUpsert:   true,
	whatsmiau.WookMessagesUpdate:   true,
	whatsmiau.WookContactsUpsert:   true,
	whatsmiau.WookConnectionUpdate: true,
	whatsmiau.WookMessagesDelete:   true,
}

type ManagerTemplates struct {
	Login     *template.Template
	Instances *template.Template
	Instance  *template.Template
}

var managerFuncMap = template.FuncMap{
	"deref": func(b *bool) bool {
		if b == nil {
			return false
		}
		return *b
	},
	"hasEvent": func(events []string, event whatsmiau.Wook) bool {
		for _, e := range events {
			if whatsmiau.Wook(e) == event {
				return true
			}
		}
		return false
	},
}

func ParseManagerTemplates() ManagerTemplates {
	base := template.Must(template.New("base").Funcs(managerFuncMap).ParseGlob("server/templates/partials/*.html"))

	login := template.Must(template.New("login").Funcs(managerFuncMap).ParseFiles("server/templates/login.html"))

	instancesBase := template.Must(template.Must(base.Clone()).ParseFiles("server/templates/layout.html"))
	instancesTmpl := template.Must(instancesBase.ParseFiles("server/templates/instances.html"))

	instanceBase := template.Must(template.Must(base.Clone()).ParseFiles("server/templates/layout.html"))
	instanceTmpl := template.Must(instanceBase.ParseFiles("server/templates/instance.html"))

	return ManagerTemplates{
		Login:     login,
		Instances: instancesTmpl,
		Instance:  instanceTmpl,
	}
}

type Manager struct {
	repo      interfaces.InstanceRepository
	whatsmiau *whatsmiau.Whatsmiau
	tmpl      ManagerTemplates
	validate  *validator.Validate
}

func NewManager(repo interfaces.InstanceRepository, w *whatsmiau.Whatsmiau, tmpl ManagerTemplates) *Manager {
	return &Manager{
		repo:      repo,
		whatsmiau: w,
		tmpl:      tmpl,
		validate:  validator.New(),
	}
}

func (s *Manager) LoginPage(ctx echo.Context) error {
	setHTMLContentType(ctx)
	return s.tmpl.Login.ExecuteTemplate(ctx.Response(), "login.html", map[string]interface{}{
		"ApiKeyRequired": len(env.Env.ApiKey) > 0,
	})
}

func (s *Manager) Login(ctx echo.Context) error {
	var req dto.ManagerLoginRequest
	if err := ctx.Bind(&req); err != nil {
		return s.renderLogin(ctx, "Erro ao processar requisição")
	}

	if len(env.Env.ApiKey) > 0 && req.ApiKey != env.Env.ApiKey {
		return s.renderLogin(ctx, "Chave de API inválida")
	}

	cookie, err := middleware.CreateSession(ctx)
	if err != nil {
		zap.L().Error("failed to create manager session", zap.Error(err))
		return s.renderLogin(ctx, "Erro interno ao criar sessão")
	}

	ctx.SetCookie(cookie)
	return ctx.Redirect(http.StatusFound, "/manager/instances")
}

func (s *Manager) renderLogin(ctx echo.Context, errMsg string) error {
	setHTMLContentType(ctx)
	return s.tmpl.Login.ExecuteTemplate(ctx.Response(), "login.html", map[string]interface{}{
		"Error":          errMsg,
		"ApiKeyRequired": len(env.Env.ApiKey) > 0,
	})
}

func (s *Manager) Logout(ctx echo.Context) error {
	token, err := middleware.SessionToken(ctx)
	if err == nil {
		expiredCookie, _ := middleware.DeleteSession(ctx, token)
		if expiredCookie != nil {
			ctx.SetCookie(expiredCookie)
		}
	}
	if isHtmx(ctx) {
		setHXRedirect(ctx, "/manager/login")
		return ctx.String(http.StatusOK, "")
	}
	return ctx.Redirect(http.StatusFound, "/manager/login")
}

func (s *Manager) ListInstances(ctx echo.Context) error {
	c := ctx.Request().Context()
	search := ctx.QueryParam("search")

	result, err := s.repo.List(c, search)
	if err != nil {
		zap.L().Error("failed to list instances", zap.Error(err))
		return s.renderInstances(ctx, search, nil, "Erro ao listar instâncias")
	}

	cards := make([]dto.ManagerInstanceCard, len(result))
	g := new(errgroup.Group)
	for i, inst := range result {
		g.Go(func() error {
			status, err := s.whatsmiau.Status(inst.ID)
			if err != nil {
				zap.L().Error("failed to get instance status", zap.String("id", inst.ID), zap.Error(err))
				status = "error"
			}
			cards[i] = dto.ManagerInstanceCard{
				ID:        inst.ID,
				RemoteJID: inst.RemoteJID,
				Status:    string(status),
			}
			return nil
		})
	}
	g.Wait()

	return s.renderInstances(ctx, search, cards, "")
}

func (s *Manager) renderInstances(ctx echo.Context, search string, cards []dto.ManagerInstanceCard, errMsg string) error {
	setHTMLContentType(ctx)
	data := map[string]interface{}{
		"Search":    search,
		"Instances": cards,
		"Error":     errMsg,
	}
	return s.tmpl.Instances.ExecuteTemplate(ctx.Response(), "instances.html", data)
}

func (s *Manager) CreateInstance(ctx echo.Context) error {
	c := ctx.Request().Context()
	var req dto.ManagerCreateInstanceRequest
	if err := ctx.Bind(&req); err != nil {
		return s.renderGridWithError(ctx, "instance_name_required")
	}

	if err := s.validate.Struct(&req); err != nil {
		return s.renderGridWithError(ctx, "instance_name_required")
	}

	if err := s.repo.Create(c, &models.Instance{ID: req.InstanceName}); err != nil {
		if errors.Is(err, instances.ErrorAlreadyExists) {
			return s.renderGridWithError(ctx, "instance_exists:"+req.InstanceName)
		}
		zap.L().Error("failed to create instance", zap.Error(err))
		return s.renderGridWithError(ctx, "instance_create_error")
	}

	setHXRedirect(ctx, "/manager/instances/"+req.InstanceName)
	return ctx.String(http.StatusOK, "")
}

func (s *Manager) renderGrid(ctx echo.Context) error {
	c := ctx.Request().Context()
	result, err := s.repo.List(c, "")
	if err != nil {
		zap.L().Error("failed to list instances", zap.Error(err))
		return s.renderGridData(ctx, nil)
	}

	cards := make([]dto.ManagerInstanceCard, len(result))
	g := new(errgroup.Group)
	for i, inst := range result {
		g.Go(func() error {
			status, err := s.whatsmiau.Status(inst.ID)
			if err != nil {
				zap.L().Error("failed to get instance status", zap.String("id", inst.ID), zap.Error(err))
				status = "error"
			}
			cards[i] = dto.ManagerInstanceCard{
				ID:        inst.ID,
				RemoteJID: inst.RemoteJID,
				Status:    string(status),
			}
			return nil
		})
	}
	g.Wait()
	return s.renderGridData(ctx, cards)
}

func (s *Manager) renderGridData(ctx echo.Context, cards []dto.ManagerInstanceCard) error {
	setHTMLContentType(ctx)
	data := map[string]interface{}{
		"Instances": cards,
	}
	return s.tmpl.Instances.ExecuteTemplate(ctx.Response(), "instances_grid.html", data)
}

func (s *Manager) renderGridWithError(ctx echo.Context, errMsg string) error {
	setHXTrigger(ctx, "showError", errMsg)
	return s.renderGrid(ctx)
}

func (s *Manager) GetInstance(ctx echo.Context) error {
	c := ctx.Request().Context()
	id := ctx.Param("id")

	result, err := s.repo.List(c, id)
	if err != nil || len(result) == 0 {
		return ctx.Redirect(http.StatusFound, "/manager/instances")
	}

	inst := result[0]
	status, err := s.whatsmiau.Status(id)
	if err != nil {
		zap.L().Error("failed to get instance status", zap.String("id", id), zap.Error(err))
		status = "error"
	}

	setHTMLContentType(ctx)
	data := map[string]interface{}{
		"Instance":            &inst,
		"Status":              string(status),
		"ID":                  id,
		"WebhookEventOptions": webhookEventOptions,
	}
	return s.tmpl.Instance.ExecuteTemplate(ctx.Response(), "instance.html", data)
}

func (s *Manager) ConnectInstance(ctx echo.Context) error {
	c := ctx.Request().Context()
	id := ctx.Param("id")

	qr, _, err := s.whatsmiau.Connect(c, id, "")
	if err != nil {
		zap.L().Error("failed to connect instance", zap.Error(err))
		setHXTrigger(ctx, "showError", "connect_error")
		return ctx.String(http.StatusOK, "")
	}

	setHTMLContentType(ctx)
	if qr == "" {
		data := map[string]interface{}{
			"Connected": true,
			"ID":        id,
		}
		return s.tmpl.Instance.ExecuteTemplate(ctx.Response(), "qrcode.html", data)
	}

	png, err := qrcode.Encode(qr, qrcode.Medium, 256)
	if err != nil {
		zap.L().Error("failed to encode qrcode", zap.Error(err))
		setHXTrigger(ctx, "showError", "qr_encode_error")
		return ctx.String(http.StatusOK, "")
	}

	data := map[string]interface{}{
		"QRBase64": base64.StdEncoding.EncodeToString(png),
		"ID":       id,
	}
	return s.tmpl.Instance.ExecuteTemplate(ctx.Response(), "qrcode.html", data)
}

func (s *Manager) PollQRCode(ctx echo.Context) error {
	id := ctx.Param("id")
	status, err := s.whatsmiau.Status(id)
	if err != nil {
		zap.L().Error("failed to get instance status", zap.String("id", id), zap.Error(err))
		return ctx.NoContent(http.StatusNoContent)
	}

	if whatsmiau.Status(status) == whatsmiau.Connected {
		setHTMLContentType(ctx)
		data := map[string]interface{}{
			"Connected": true,
			"ID":        id,
		}
		return s.tmpl.Instance.ExecuteTemplate(ctx.Response(), "qrcode.html", data)
	}

	/*
	 * For any other state (qr-code, connecting, closed), keep the QR displayed.
	 * Avoids false "expired" due to transient status during connection.
	 * Expiration is handled client-side with a 2 min setTimeout.
	 */
	return ctx.NoContent(http.StatusNoContent)
}

func (s *Manager) LogoutInstance(ctx echo.Context) error {
	c := ctx.Request().Context()
	id := ctx.Param("id")

	if err := s.whatsmiau.Logout(c, id); err != nil {
		zap.L().Error("failed to logout instance", zap.Error(err))
		setHXTrigger(ctx, "showError", "disconnect_error")
		return ctx.String(http.StatusOK, "")
	}

	if isHtmx(ctx) {
		setHXRedirect(ctx, "/manager/instances/"+id)
		return ctx.String(http.StatusOK, "")
	}
	return ctx.Redirect(http.StatusFound, "/manager/instances/"+id)
}

func (s *Manager) DeleteInstance(ctx echo.Context) error {
	c := ctx.Request().Context()
	id := ctx.Param("id")

	result, err := s.repo.List(c, id)
	if err != nil {
		zap.L().Error("failed to list instances", zap.String("id", id), zap.Error(err))
		setHXTrigger(ctx, "showError", "delete_error")
		return ctx.String(http.StatusOK, "")
	}

	if len(result) == 0 {
		setHXTrigger(ctx, "showError", "instance_not_found")
		return ctx.String(http.StatusOK, "")
	}

	if err := s.whatsmiau.Logout(c, id); err != nil {
		zap.L().Error("failed to disconnect instance", zap.String("id", id), zap.Error(err))
		setHXTrigger(ctx, "showError", "disconnect_error")
		return ctx.String(http.StatusOK, "")
	}

	if err := s.repo.Delete(c, id); err != nil {
		zap.L().Error("failed to delete instance", zap.Error(err))
		setHXTrigger(ctx, "showError", "delete_error")
		return ctx.String(http.StatusOK, "")
	}

	if isHtmx(ctx) {
		setHXRedirect(ctx, "/manager/instances")
		return ctx.String(http.StatusOK, "")
	}
	return ctx.Redirect(http.StatusFound, "/manager/instances")
}

func (s *Manager) UpdateInstance(ctx echo.Context) error {
	c := ctx.Request().Context()
	id := ctx.Param("id")

	var req dto.ManagerUpdateInstanceRequest
	if err := ctx.Bind(&req); err != nil {
		setHXTrigger(ctx, "showError", "form_error")
		return ctx.String(http.StatusOK, "")
	}

	if err := s.validate.Struct(&req); err != nil {
		setHXTrigger(ctx, "showError", "form_error")
		return ctx.String(http.StatusOK, "")
	}

	webhookByEvents := req.WebhookByEvents != nil
	webhookBase64 := req.WebhookBase64 != nil

	toUpdate := &models.Instance{
		Webhook: models.InstanceWebhook{
			Url:      req.WebhookURL,
			ByEvents: &webhookByEvents,
			Base64:   &webhookBase64,
			Events:   req.WebhookEvents,
		},
		InstanceProxy: models.InstanceProxy{
			ProxyHost:     req.ProxyHost,
			ProxyPort:     req.ProxyPort,
			ProxyProtocol: req.ProxyProtocol,
			ProxyUsername: req.ProxyUsername,
			ProxyPassword: req.ProxyPassword,
		},
	}

	if _, err := s.repo.Update(c, id, toUpdate); err != nil {
		if errors.Is(err, instances.ErrorNotFound) {
			setHXTrigger(ctx, "showError", "instance_not_found")
			return ctx.String(http.StatusOK, "")
		}
		zap.L().Error("failed to update instance", zap.Error(err))
		setHXTrigger(ctx, "showError", "save_error")
		return ctx.String(http.StatusOK, "")
	}

	setHXTrigger(ctx, "showSuccess", "settings_saved")
	return ctx.String(http.StatusOK, "")
}

func (s *Manager) StatusBadge(ctx echo.Context) error {
	id := ctx.Param("id")
	status, err := s.whatsmiau.Status(id)
	if err != nil {
		zap.L().Error("failed to get instance status", zap.String("id", id), zap.Error(err))
		status = "error"
	}

	setHTMLContentType(ctx)
	data := map[string]interface{}{
		"ID":     id,
		"Status": string(status),
	}
	return s.tmpl.Instance.ExecuteTemplate(ctx.Response(), "status_badge.html", data)
}

func setHXTrigger(ctx echo.Context, event, msg string) {
	payload, _ := json.Marshal(map[string]string{event: msg})
	ctx.Response().Header().Set("HX-Trigger", string(payload))
}

func isHtmx(ctx echo.Context) bool {
	return ctx.Request().Header.Get("HX-Request") == "true"
}

func setHXRedirect(ctx echo.Context, url string) {
	ctx.Response().Header().Set("HX-Redirect", url)
}

func setHTMLContentType(ctx echo.Context) {
	ctx.Response().Header().Set("Content-Type", "text/html; charset=utf-8")
}

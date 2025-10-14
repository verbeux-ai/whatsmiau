package controllers

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/verbeux-ai/whatsmiau/models"
	"go.mau.fi/whatsmeow/types"
)

func numberToJid(number string) (*types.JID, error) {
	splitNumber := strings.Split(number, "@")
	if len(splitNumber) != 2 {
		number += "@s.whatsapp.net"
	}

	if len(splitNumber[0]) < 12 {
		return nil, fmt.Errorf("invalid jid, put country prefix")
	}

	jid, err := types.ParseJID(number)
	if err != nil {
		return nil, fmt.Errorf("invalid jid (number)")
	}

	return &jid, nil
}

func parseProxyURL(proxyURL string) (*models.InstanceProxy, error) {
	if !strings.Contains(proxyURL, "://") {
		return nil, fmt.Errorf("invalid proxy url, missing scheme: %s", proxyURL)
	}

	u, err := url.Parse(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse proxy url: %w", err)
	}

	var username, password string
	if u.User != nil {
		username = u.User.Username()
		password, _ = u.User.Password()
	}

	host, port, err := splitHostPort(u.Host)
	if err != nil {
		return nil, fmt.Errorf("invalid host/port: %w", err)
	}

	return &models.InstanceProxy{
		ProxyHost:     host,
		ProxyPort:     port,
		ProxyProtocol: strings.ToUpper(u.Scheme),
		ProxyUsername: username,
		ProxyPassword: password,
	}, nil
}

func splitHostPort(h string) (string, string, error) {
	parts := strings.Split(h, ":")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("expected host:port, got %s", h)
	}
	return parts[0], parts[1], nil
}

//go:build integration

package integration_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// cooldown aguarda entre operações que criam grupos para evitar rate-limit do WhatsApp.
func cooldown() { time.Sleep(3 * time.Second) }

var httpClient = &http.Client{Timeout: 30 * time.Second}

func baseURL() string {
	if u := os.Getenv("TEST_BASE_URL"); u != "" {
		return u
	}
	return "http://localhost:8080"
}

func apiKey() string {
	return os.Getenv("TEST_API_KEY")
}

// instanceID retorna TEST_INSTANCE_ID e falha o teste se não estiver definida.
func instanceID(t *testing.T) string {
	t.Helper()
	id := os.Getenv("TEST_INSTANCE_ID")
	require.NotEmpty(t, id, "TEST_INSTANCE_ID deve estar definida para testes de integração")
	return id
}

// participantNumber retorna TEST_PARTICIPANT_NUMBER (opcional).
// Testes que dependem desse valor devem chamar t.Skip se estiver vazio.
func participantNumber() string {
	return os.Getenv("TEST_PARTICIPANT_NUMBER")
}

// do executa uma requisição HTTP com auth e Content-Type configurados.
func do(t *testing.T, method, path string, body any) *http.Response {
	t.Helper()
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, baseURL()+path, r)
	require.NoError(t, err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if k := apiKey(); k != "" {
		req.Header.Set("apikey", k)
	}
	resp, err := httpClient.Do(req)
	require.NoError(t, err)
	return resp
}

// mustDecode decodifica o body JSON da resposta em out.
func mustDecode(t *testing.T, resp *http.Response, out any) {
	t.Helper()
	defer resp.Body.Close()
	require.NoError(t, json.NewDecoder(resp.Body).Decode(out))
}

// drainClose descarta e fecha o body da resposta.
func drainClose(resp *http.Response) {
	io.Copy(io.Discard, resp.Body) //nolint:errcheck
	resp.Body.Close()
}

func groupURL(t *testing.T, action string) string {
	return fmt.Sprintf("/v1/instance/%s/group/%s", instanceID(t), action)
}

func groupURLQuery(t *testing.T, action, query string) string {
	return fmt.Sprintf("/v1/instance/%s/group/%s?%s", instanceID(t), action, query)
}

func communityURL(t *testing.T, action string) string {
	return fmt.Sprintf("/v1/instance/%s/community/%s", instanceID(t), action)
}

func communityURLQuery(t *testing.T, action, query string) string {
	return fmt.Sprintf("/v1/instance/%s/community/%s?%s", instanceID(t), action, query)
}

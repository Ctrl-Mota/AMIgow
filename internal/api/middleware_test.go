package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/safehouse/amigow/internal/config"
)

func TestValidateAPIKeyAllowsQueryOnlyFromLoopback(t *testing.T) {
	original := config.Current
	t.Cleanup(func() { config.Current = original })
	config.Current = &config.Config{APIKey: "local-secret"}

	h := &Handler{}
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	local := httptest.NewRequest(http.MethodGet, "/portaria/wakeup?api_key=local-secret", nil)
	local.RemoteAddr = "127.0.0.1:54321"
	localResponse := httptest.NewRecorder()
	h.ValidateAPIKey(next).ServeHTTP(localResponse, local)
	if localResponse.Code != http.StatusNoContent {
		t.Fatalf("loopback deveria aceitar api_key na query; status=%d", localResponse.Code)
	}

	remote := httptest.NewRequest(http.MethodGet, "/portaria/wakeup?api_key=local-secret", nil)
	remote.RemoteAddr = "192.0.2.10:54321"
	remoteResponse := httptest.NewRecorder()
	h.ValidateAPIKey(next).ServeHTTP(remoteResponse, remote)
	if remoteResponse.Code != http.StatusUnauthorized {
		t.Fatalf("rede não deve aceitar api_key na query; status=%d", remoteResponse.Code)
	}
}

package api

import (
	"log"
	"net"
	"net/http"
	"strings"

	"github.com/safehouse/amigow/internal/config"
)

func (h *Handler) ValidateAPIKey(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("X-API-Key")
		// O pbx_lua usa um helper HTTP somente-GET. Permita a chave na query
		// apenas no hop local Asterisk -> AMIgow; nunca aceite isso da rede.
		if apiKey == "" && isLoopbackRequest(r) {
			apiKey = r.URL.Query().Get("api_key")
		}

		if strings.Contains(r.URL.Path, "/docs") ||
			strings.Contains(r.URL.Path, "/openapi.json") ||
			strings.Contains(r.URL.Path, "/health") ||
			strings.Contains(r.URL.Path, "/webhooks/schema") {
			next.ServeHTTP(w, r)
			return
		}

		if apiKey == "" {
			log.Printf("[API] Tentativa de acesso sem API key")
			respondError(w, http.StatusUnauthorized, "API key é obrigatória")
			return
		}

		if apiKey != config.Current.APIKey {
			log.Printf("[API] Tentativa de acesso com API key inválida")
			respondError(w, http.StatusUnauthorized, "API key inválida")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func isLoopbackRequest(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

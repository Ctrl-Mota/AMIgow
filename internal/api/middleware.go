package api

import (
	"log"
	"net/http"
	"strings"

	"github.com/safehouse/amigow/internal/config"
)

func (h *Handler) ValidateAPIKey(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("X-API-Key")

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

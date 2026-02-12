package api

import (
	"log"
	"net/http"
	"strings"
)

func (h *Handler) ValidateAPIKey(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("X-API-Key")

		if strings.Contains(r.URL.Path, "/docs") || strings.Contains(r.URL.Path, "/openapi.json") {
			next.ServeHTTP(w, r)
			return
		}

		if apiKey == "" {
			log.Printf("[API] Tentativa de acesso sem API key")
			respondError(w, http.StatusUnauthorized, "API key é obrigatória")
			return
		}

		if apiKey != h.Config.APIKey {
			log.Printf("[API] Tentativa de acesso com API key inválida")
			respondError(w, http.StatusUnauthorized, "API key inválida")
			return
		}

		next.ServeHTTP(w, r)
	})
}

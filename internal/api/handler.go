package api

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/safehouse/amigow/internal/ami"
)

type Handler struct {
	Managers map[string]*ami.AsteriskManager
}

func NewHandler(managers map[string]*ami.AsteriskManager) *Handler {
	return &Handler{
		Managers: managers,
	}
}

func (h *Handler) HandleAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	asteriskID := r.Header.Get("X-Asterisk-ID")
	if asteriskID == "" {
		http.Error(w, "Header X-Asterisk-ID é obrigatório", http.StatusBadRequest)
		return
	}

	manager, found := h.Managers[asteriskID]
	if !found {
		http.Error(w, "Asterisk ID não encontrado", http.StatusNotFound)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("[API] Erro ao ler body: %v", err)
		http.Error(w, "Erro ao ler requisição", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var action map[string]string
	err = json.Unmarshal(body, &action)
	if err != nil {
		log.Printf("[API] Erro ao decodificar JSON: %v", err)
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	if action["Action"] == "" {
		http.Error(w, "Campo Action é obrigatório", http.StatusBadRequest)
		return
	}

	log.Printf("[API] Executando action %s para %s", action["Action"], asteriskID)

	response, err := manager.SendAction(action)
	if err != nil {
		log.Printf("[API] Erro ao executar action: %v", err)

		errorResponse := map[string]string{
			"error": err.Error(),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errorResponse)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func (h *Handler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"status":   "ok",
		"managers": len(h.Managers),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

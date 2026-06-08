package api

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/safehouse/amigow/internal/ami"
	"github.com/safehouse/amigow/internal/config"
	"github.com/safehouse/amigow/internal/queues"
)

type Handler struct {
	Manager *ami.AsteriskManager
	Config  *config.Config
	CDRDB   *sql.DB
	Store   *queues.SnapshotStore
}

func NewHandler(manager *ami.AsteriskManager, cdrDB *sql.DB, store *queues.SnapshotStore) *Handler {
	return &Handler{
		Manager: manager,
		CDRDB:   cdrDB,
		Store:   store,
	}
}

type ActionInput struct {
	Body ActionRequest
}

func (h *Handler) HandleAction(ctx context.Context, input *ActionInput) (*ActionResponse, error) {
	if input.Body.Action["Action"] == "" {
		return nil, errorBadRequest("Campo Action é obrigatório")
	}

	log.Printf("[API] Executando action %s", input.Body.Action["Action"])

	response, err := h.Manager.SendAction(input.Body.Action)
	if err != nil {
		log.Printf("[API] Erro ao executar action: %v", err)
		return nil, errorInternal(fmt.Sprintf("Erro ao executar ação: %v", err))
	}

	result := &ActionResponse{}
	result.Body.Response = response
	return result, nil
}

func (h *Handler) HandleHealth(ctx context.Context, input *struct{}) (*HealthResponse, error) {
	result := &HealthResponse{}
	result.Body.Status = "ok"
	return result, nil
}

func (h *Handler) HandleWebhookSchema(ctx context.Context, input *struct{}) (*WebhookSchemaResponse, error) {
	result := &WebhookSchemaResponse{}
	result.Body.Description = "O AMIgow envia webhooks para URLs configuradas quando eventos AMI ocorrem"
	result.Body.Payload = WebhookCallbackPayload{
		EventType:   "answer",
		Timestamp:   "2026-02-10T17:00:00Z",
		Channel:     "SIP/1001-0000001",
		CallerID:    "1001",
		CallerName:  "João Silva",
		Destination: "2000",
		Cause:       "16",
		CauseText:   "Normal clearing",
		Duration:    "45",
		RawData:     map[string]string{"Event": "Newchannel", "Channel": "SIP/1001-0000001"},
	}
	result.Body.Headers = map[string]string{
		"Content-Type": "application/json",
		"X-API-Key":    "api-key-1", // ID do servidor Asterisk que enviou o evento
	}
	return result, nil
}

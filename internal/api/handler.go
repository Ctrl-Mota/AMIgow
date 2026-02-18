package api

import (
	"context"
	"fmt"
	"log"

	"github.com/safehouse/amigow/internal/ami"
	"github.com/safehouse/amigow/internal/config"
)

type Handler struct {
	Managers map[string]*ami.AsteriskManager
	Config   *config.Config
}

func NewHandler(managers map[string]*ami.AsteriskManager, cfg *config.Config) *Handler {
	return &Handler{
		Managers: managers,
		Config:   cfg,
	}
}

type ActionInput struct {
	Body       ActionRequest
	AsteriskID string `header:"X-Asterisk-ID" doc:"ID do servidor Asterisk"`
}

func (h *Handler) HandleAction(ctx context.Context, input *ActionInput) (*ActionResponse, error) {
	if input.AsteriskID == "" {
		return nil, errorBadRequest("Header X-Asterisk-ID é obrigatório")
	}

	manager, found := h.Managers[input.AsteriskID]
	if !found {
		return nil, errorNotFound("Asterisk ID não encontrado")
	}

	if input.Body.Action["Action"] == "" {
		return nil, errorBadRequest("Campo Action é obrigatório")
	}

	log.Printf("[API] Executando action %s para %s", input.Body.Action["Action"], input.AsteriskID)

	response, err := manager.SendAction(input.Body.Action)
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
	result.Body.Managers = len(h.Managers)
	return result, nil
}

func (h *Handler) HandleWebhookSchema(ctx context.Context, input *struct{}) (*WebhookSchemaResponse, error) {
	result := &WebhookSchemaResponse{}
	result.Body.Description = "O AMIgow envia webhooks para URLs configuradas quando eventos AMI ocorrem"
	result.Body.Payload = WebhookCallbackPayload{
		EventType:   "answer",
		Source:      "teste-1",
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
		"Content-Type":  "application/json",
		"X-Asterisk-ID": "teste-1", // ID do servidor Asterisk que enviou o evento
	}
	return result, nil
}

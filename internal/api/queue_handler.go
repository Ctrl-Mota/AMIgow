package api

import (
	"context"
	"fmt"
	"log"

	"github.com/safehouse/amigow/internal/ami"
)

type QueueInput struct {
	Body       QueueRequest
	AsteriskID string `header:"X-Asterisk-ID" doc:"ID do servidor Asterisk"`
}

func (h *Handler) HandleQueueAdd(ctx context.Context, input *QueueInput) (*QueueResponse, error) {
	if input.AsteriskID == "" {
		return nil, errorBadRequest("Header X-Asterisk-ID é obrigatório")
	}

	manager, found := h.Managers[input.AsteriskID]
	if !found {
		return nil, errorNotFound("Asterisk ID não encontrado")
	}

	if input.Body.Queue == "" || input.Body.Interface == "" {
		return nil, errorBadRequest("Campos queue e interface são obrigatórios")
	}

	log.Printf("[API] Adicionando interface %s à fila %s", input.Body.Interface, input.Body.Queue)

	bgCtx := context.Background()
	queueData := ami.QueueData{
		Queue:     input.Body.Queue,
		Interface: input.Body.Interface,
	}

	response, err := ami.SendQueueAdd(bgCtx, manager, queueData)
	if err != nil {
		log.Printf("[API] Erro ao adicionar à fila: %v", err)
		return nil, errorBadRequest(fmt.Sprintf("Erro ao adicionar à fila: %v", err))
	}
	if response.Get("Message") != "Added interface to queue" {
		return nil, errorBadRequest(response.Get("Message"))
	}

	result := &QueueResponse{}
	result.Body.Response = "Success"
	result.Body.Message = response.Get("Message")
	return result, nil
}

func (h *Handler) HandleQueueRemove(ctx context.Context, input *QueueInput) (*QueueResponse, error) {
	if input.AsteriskID == "" {
		return nil, errorBadRequest("Header X-Asterisk-ID é obrigatório")
	}

	manager, found := h.Managers[input.AsteriskID]
	if !found {
		return nil, errorNotFound("Asterisk ID não encontrado")
	}

	if input.Body.Queue == "" || input.Body.Interface == "" {
		return nil, errorBadRequest("Campos queue e interface são obrigatórios")
	}

	log.Printf("[API] Removendo interface %s da fila %s", input.Body.Interface, input.Body.Queue)

	bgCtx := context.Background()
	response, err := ami.SendQueueRemove(bgCtx, manager, input.Body.Queue, input.Body.Interface)
	if err != nil {
		log.Printf("[API] Erro ao remover da fila: %v", err)
		return nil, errorBadRequest(fmt.Sprintf("Erro ao remover da fila: %v", err))
	}
	if response.Get("Message") != "Removed interface from queue" {
		return nil, errorBadRequest(response.Get("Message"))
	}
	result := &QueueResponse{}
	result.Body.Response = "Success"
	result.Body.Message = response.Get("Message")
	return result, nil
}

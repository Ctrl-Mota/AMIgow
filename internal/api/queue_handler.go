package api

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/safehouse/amigow/internal/ami"
)

type QueueInput struct {
	Body QueueRequest
}
type QueueStatusInput struct {
	Body QueueStatusRequest
}

func (h *Handler) HandleQueueAdd(ctx context.Context, input *QueueInput) (*QueueResponse, error) {
	if input.Body.Queue == "" || input.Body.Interface == "" {
		return nil, errorBadRequest("Campos queue e interface são obrigatórios")
	}

	log.Printf("[API] Adicionando interface %s à fila %s", input.Body.Interface, input.Body.Queue)

	bgCtx := context.Background()
	queueData := ami.QueueData{
		Queue:     input.Body.Queue,
		Interface: input.Body.Interface,
	}

	response, err := ami.SendQueueAdd(bgCtx, h.Manager, queueData)
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
	if input.Body.Queue == "" || input.Body.Interface == "" {
		return nil, errorBadRequest("Campos queue e interface são obrigatórios")
	}

	log.Printf("[API] Removendo interface %s da fila %s", input.Body.Interface, input.Body.Queue)

	bgCtx := context.Background()
	response, err := ami.SendQueueRemove(bgCtx, h.Manager, input.Body.Queue, input.Body.Interface)
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

func (h *Handler) HandleQueueStatus(ctx context.Context, input *QueueStatusInput) (*QueueResponse, error) {
	if input.Body.Queue == "" || input.Body.Interfaces == "" {
		return nil, errorBadRequest("Campos queue e interface são obrigatórios")
	}

	log.Printf("[API] Obtendo status das interfaces %s da fila %s", input.Body.Interfaces, input.Body.Queue)

	bgCtx := context.Background()
	ami.SendQueueStatus(bgCtx, h.Manager, input.Body.Queue)
	interfaces := strings.Split(input.Body.Interfaces, ",")
	for _, iface := range interfaces {
		_, err := ami.SendQueueMemberStatus(bgCtx, h.Manager, input.Body.Queue, iface)
		if err != nil {
			log.Printf("[API] Erro ao obter status da interface %s: %v", iface, err)
			return nil, errorBadRequest(fmt.Sprintf("Erro ao obter status da interface %s: %v", iface, err))
		}
	}

	result := &QueueResponse{}
	result.Body.Response = "Success"
	result.Body.Message = "Status solicitado"

	return result, nil
}

package api

import (
	"context"
	"fmt"
	"log"

	"github.com/safehouse/amigow/internal/ami"
)

type ChannelRedirectInput struct {
	Body ChannelRedirectRequest
}

func (h *Handler) HandleChannelRedirect(ctx context.Context, input *ChannelRedirectInput) (*ChannelRedirectResponse, error) {
	if input.Body.Channel == "" || input.Body.Exten == "" || input.Body.Context == "" || input.Body.Priority == "" {
		return nil, errorBadRequest("Campos channel, exten, context e priority são obrigatórios")
	}

	log.Printf("[API] Redirecionando canal %s para ramal %s@%s", input.Body.Channel, input.Body.Exten, input.Body.Context)

	channelData := ami.ChannelData{
		Channel:  input.Body.Channel,
		Exten:    input.Body.Exten,
		Context:  input.Body.Context,
		Priority: input.Body.Priority,
	}

	response, err := ami.SendChannelRedirect(ctx, h.Manager, channelData)
	if err != nil {
		log.Printf("[API] Erro ao redirecionar canal: %v", err)
		return nil, errorBadRequest(fmt.Sprintf("Erro ao redirecionar canal: %v", err))
	}

	result := &ChannelRedirectResponse{}
	result.Body.Response = "Success"
	result.Body.Message = response.Get("Message")
	return result, nil
}

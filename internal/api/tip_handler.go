package api

import (
	"context"
	"fmt"
	"log"
	"net"
	"os/exec"
	"time"
)

type TipInput struct {
	IP string `query:"ip" required:"true" doc:"IP a ser confiado" example:"192.168.0.10"`
}

type TipResponse struct {
	Body struct {
		Output string `json:"output" doc:"Saída do script tip.sh"`
	}
}

func (h *Handler) HandleTip(ctx context.Context, input *TipInput) (*TipResponse, error) {
	if net.ParseIP(input.IP) == nil {
		return nil, errorBadRequest("Parâmetro ip inválido")
	}

	log.Printf("[API] tip.sh %s", input.IP)

	execCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	out, err := exec.CommandContext(execCtx, "/opt/amigow/tip.sh", input.IP).CombinedOutput()
	if err != nil {
		return nil, errorInternal(fmt.Sprintf("Falha ao executar tip.sh: %v | saída: %s", err, string(out)))
	}

	result := &TipResponse{}
	result.Body.Output = string(out)
	return result, nil
}

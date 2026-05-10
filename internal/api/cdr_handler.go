package api

import (
	"context"
	"fmt"
	"log"

	"github.com/safehouse/amigow/internal/cdr"
)

func (h *Handler) HandleCDRSearch(ctx context.Context, input *CDRSearchInput) (*CDRSearchResponse, error) {
	if input.Linkedid == "" {
		return nil, errorBadRequest("parâmetro linkedid é obrigatório")
	}

	if h.CDRDB == nil {
		return nil, errorInternal("banco CDR não configurado")
	}

	table := h.Config.CDRDB.Table
	if table == "" {
		table = "cdr"
	}

	log.Printf("[CDR] Buscando registros para linkedid=%s", input.Linkedid)

	rows, err := cdr.SearchByLinkedID(ctx, h.CDRDB, table, input.Linkedid)
	if err != nil {
		log.Printf("[CDR] Erro ao buscar CDR: %v", err)
		return nil, errorInternal(fmt.Sprintf("erro ao buscar CDR: %v", err))
	}

	result := &CDRSearchResponse{}
	result.Body.Rows = rows
	return result, nil
}

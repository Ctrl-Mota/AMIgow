package api

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/safehouse/amigow/internal/queues"
)

type QueuesDashboardInput struct {
	Since       uint64 `query:"since" doc:"Vers\u00e3o conhecida; retorna 304 se igual"`
	IfNoneMatch string `header:"If-None-Match"`
}

type QueuesDashboardOutput struct {
	Status int    `header:"-"`
	ETag   string `header:"ETag"`
	Body   queues.DashboardSnapshot
}

func (h *Handler) HandleQueuesDashboard(ctx context.Context, in *QueuesDashboardInput) (*QueuesDashboardOutput, error) {
	if h.Store == nil {
		return nil, errorInternal("dashboard de queues n\u00e3o habilitado")
	}

	current := h.Store.Version()
	currentETag := fmt.Sprintf("\"queues-dashboard-%d\"", current)
	if in.IfNoneMatch != "" && stripETag(in.IfNoneMatch) == stripETag(currentETag) {
		log.Printf("[DASH] GET dashboard since=%d current=%d resp=304 (etag)", in.Since, current)
		return &QueuesDashboardOutput{Status: 304, ETag: currentETag}, nil
	}

	changed, snap := h.Store.SnapshotSince(in.Since)
	if !changed {
		log.Printf("[DASH] GET dashboard since=%d current=%d resp=304", in.Since, snap.Version)
		return &QueuesDashboardOutput{
			Status: 304,
			ETag:   fmt.Sprintf("\"queues-dashboard-%d\"", snap.Version),
		}, nil
	}

	log.Printf("[DASH] GET dashboard since=%d current=%d resp=200", in.Since, snap.Version)
	return &QueuesDashboardOutput{
		Status: 200,
		ETag:   fmt.Sprintf("\"queues-dashboard-%d\"", snap.Version),
		Body:   snap,
	}, nil
}

func stripETag(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "W/")
	s = strings.Trim(s, "\"")
	return s
}

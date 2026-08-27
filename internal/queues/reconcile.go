package queues

import (
	"context"
	"log"
	"time"

	"github.com/safehouse/amigow/internal/ami"
)

const reconcileQueryTimeout = 20 * time.Second

func RunReconcile(ctx context.Context, mgr *ami.AsteriskManager, store *SnapshotStore, every time.Duration) {
	if every <= 0 {
		every = 60 * time.Second
	}

	log.Printf("[DASH] reconcile iniciado (intervalo=%s)", every)
	ticker := time.NewTicker(every)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("[DASH] reconcile encerrado")
			return
		case <-ticker.C:
			queryCtx, cancel := context.WithTimeout(ctx, reconcileQueryTimeout)
			events, err := ami.SendQueueStatusAll(queryCtx, mgr)
			cancel()
			if err != nil {
				log.Printf("[DASH] reconcile: erro QueueStatuses: %v", err)
				store.MarkAMIConnected(false)
				continue
			}
			store.MarkAMIConnected(true)
			store.ApplyQueueStatuses(events)
		}
	}
}

package queues

import (
	"context"
	"log"
	"time"
)

// RunLiveTick mantem a `version` do snapshot crescendo enquanto houver
// callers esperando ou agents ringing. Isso garante que clientes em polling
// curto (ex: 1s) recebam updates do wait_seconds calculado dinamicamente,
// mesmo sem novos eventos AMI. Sem atividade vira no-op (mantem 304).
func RunLiveTick(ctx context.Context, store *SnapshotStore, interval time.Duration) {
	if interval <= 0 {
		interval = time.Second
	}
	log.Printf("[DASH] live tick iniciado (intervalo=%s)", interval)
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Println("[DASH] live tick encerrado")
			return
		case <-t.C:
			store.BumpIfLive()
		}
	}
}

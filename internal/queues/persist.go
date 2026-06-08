package queues

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/safehouse/amigow/internal/ami"
)

var eventsFileMu sync.Mutex

type PersistConfig struct {
	SnapshotPath    string
	EventsPath      string
	IntervalSeconds int
	// Dirty recebe um pulso (estilo signal) sempre que o reducer aplica uma
	// mutacao que muda a `version` do store. RunPersist usa esses pulsos para
	// escrever snapshot.json com debounce de 250ms, garantindo que o disco
	// reflita o estado vivo quase imediatamente sem fritar I/O em rajadas.
	Dirty <-chan struct{}
}

func RunPersist(ctx context.Context, store *SnapshotStore, cfg PersistConfig) {
	if cfg.SnapshotPath == "" {
		log.Println("[DASH] persist desabilitado (snapshot_path vazio)")
		return
	}

	interval := time.Duration(cfg.IntervalSeconds) * time.Second
	if interval <= 0 {
		interval = 5 * time.Second
	}

	if err := ensureDir(cfg.SnapshotPath); err != nil {
		log.Printf("[DASH] persist: erro ao criar diret\u00f3rio do snapshot: %v", err)
	}
	if cfg.EventsPath != "" {
		if err := ensureDir(cfg.EventsPath); err != nil {
			log.Printf("[DASH] persist: erro ao criar diret\u00f3rio de eventos: %v", err)
		}
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	debounce := time.NewTimer(time.Hour)
	if !debounce.Stop() {
		<-debounce.C
	}
	debouncing := false
	defer debounce.Stop()

	writeSnapshot(cfg.SnapshotPath, store)

	for {
		select {
		case <-ctx.Done():
			log.Println("[DASH] persist encerrado")
			writeSnapshot(cfg.SnapshotPath, store)
			return
		case <-ticker.C:
			writeSnapshot(cfg.SnapshotPath, store)
		case <-cfg.Dirty:
			if !debouncing {
				debounce.Reset(250 * time.Millisecond)
				debouncing = true
			}
		case <-debounce.C:
			debouncing = false
			writeSnapshot(cfg.SnapshotPath, store)
		}
	}
}

func writeSnapshot(path string, store *SnapshotStore) {
	snap := store.Snapshot()
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		log.Printf("[DASH] persist: erro ao serializar snapshot: %v", err)
		return
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		log.Printf("[DASH] persist: erro ao escrever tmp: %v", err)
		return
	}
	if err := os.Rename(tmp, path); err != nil {
		log.Printf("[DASH] persist: erro ao renomear snapshot: %v", err)
	}
}

func AppendEvent(basePath string, ev ami.Event) {
	eventsFileMu.Lock()
	defer eventsFileMu.Unlock()

	path := eventsPathForDay(basePath, ev.Timestamp)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		log.Printf("[DASH] persist: erro ao abrir events: %v", err)
		return
	}
	defer f.Close()

	record := map[string]any{
		"occurred_at": ev.Timestamp.Format(time.RFC3339),
		"event":       ev.Type,
		"source":      ev.Source,
		"queue":       ev.Data["Queue"],
		"uniqueid":    ev.Data["Uniqueid"],
		"linkedid":    ev.Data["Linkedid"],
		"callerid_num": ev.Data["CallerIDNum"],
		"hold_time":   atoiSafe(ev.Data["HoldTime"]),
		"talk_time":   atoiSafe(ev.Data["TalkTime"]),
		"position":    atoiSafe(ev.Data["Position"]),
		"interface":   ev.Data["Interface"],
		"payload":     ev.Data,
	}

	data, err := json.Marshal(record)
	if err != nil {
		log.Printf("[DASH] persist: erro ao serializar event: %v", err)
		return
	}
	if _, err := f.Write(append(data, '\n')); err != nil {
		log.Printf("[DASH] persist: erro ao escrever event: %v", err)
		return
	}
	_ = f.Sync()
}

func LoadSnapshot(path string) (DashboardSnapshot, bool) {
	if path == "" {
		return DashboardSnapshot{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return DashboardSnapshot{}, false
	}
	var snap DashboardSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		log.Printf("[DASH] persist: snapshot inv\u00e1lido em %s: %v", path, err)
		return DashboardSnapshot{}, false
	}
	snap.Stale = true
	return snap, true
}

func ensureDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "" || dir == "." {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

// eventsPathForDay devolve <stem>.YYYY-MM-DD<ext> a partir do base configurado.
// Ex: base=/var/lib/amigow/queue-dashboard.events.ndjson
//     dia=2026-06-03 -> /var/lib/amigow/queue-dashboard.events.2026-06-03.ndjson
func eventsPathForDay(base string, t time.Time) string {
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	return fmt.Sprintf("%s.%s%s", stem, t.Format("2006-01-02"), ext)
}

// RunEventsRetention varre o diretorio do arquivo de eventos a cada hora e
// apaga arquivos rotacionados (<stem>.YYYY-MM-DD<ext>) com idade > keepDays.
func RunEventsRetention(ctx context.Context, basePath string, keepDays int) {
	if basePath == "" {
		log.Println("[DASH] retention desabilitado (events_path vazio)")
		return
	}
	if keepDays <= 0 {
		keepDays = 7
	}
	log.Printf("[DASH] retention iniciado (mantendo %d dias)", keepDays)

	// Roda 1x no boot para limpar restos.
	pruneOldEvents(basePath, keepDays)

	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Println("[DASH] retention encerrado")
			return
		case <-ticker.C:
			pruneOldEvents(basePath, keepDays)
		}
	}
}

var dayInFilenameRe = regexp.MustCompile(`\.(\d{4}-\d{2}-\d{2})\.`)

func pruneOldEvents(basePath string, keepDays int) {
	dir := filepath.Dir(basePath)
	if dir == "" {
		dir = "."
	}
	ext := filepath.Ext(basePath)
	stem := strings.TrimSuffix(filepath.Base(basePath), ext)
	prefix := stem + "."

	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Printf("[DASH] retention: erro ao listar %s: %v", dir, err)
		return
	}

	cutoff := time.Now().AddDate(0, 0, -keepDays)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, ext) {
			continue
		}
		match := dayInFilenameRe.FindStringSubmatch(name)
		if len(match) != 2 {
			continue
		}
		day, err := time.ParseInLocation("2006-01-02", match[1], time.Local)
		if err != nil {
			continue
		}
		if day.Before(cutoff) {
			full := filepath.Join(dir, name)
			if err := os.Remove(full); err != nil {
				log.Printf("[DASH] retention: erro ao remover %s: %v", full, err)
			} else {
				log.Printf("[DASH] retention: removido %s", full)
			}
		}
	}
}

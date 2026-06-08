package queues

import (
	"strconv"
	"sync"
	"time"

	goami "github.com/heltonmarx/goami/ami"
	"github.com/safehouse/amigow/internal/ami"
	"github.com/safehouse/amigow/internal/freepbx"
)

type queueState struct {
	ID                 string
	Name               string
	Strategy           string
	Config             QueueConfigBlock
	Callers            map[string]*CallerSnapshot
	Members            map[string]*AgentSnapshot
	LastEventAt        time.Time
	LongestWaitSeconds int
}

type SnapshotStore struct {
	mu                 sync.RWMutex
	version            uint64
	queues             map[string]*queueState
	metrics            *RollingMetrics
	lastEventAt        time.Time
	amiOK              bool
	reconnects         int
	serviceLevelTarget int
	pollingRecommended int
	pollingMin         int
	alertThresholds    AlertThresholds
}

func New(serviceLevelTarget int, pollingRecommendedMs int, pollingMinMs int, thresholds AlertThresholds) *SnapshotStore {
	if serviceLevelTarget <= 0 {
		serviceLevelTarget = 20
	}
	if pollingRecommendedMs <= 0 {
		pollingRecommendedMs = 2000
	}
	if pollingMinMs <= 0 {
		pollingMinMs = 1000
	}
	return &SnapshotStore{
		queues:             make(map[string]*queueState),
		metrics:            NewRollingMetrics(),
		serviceLevelTarget: serviceLevelTarget,
		pollingRecommended: pollingRecommendedMs,
		pollingMin:         pollingMinMs,
		alertThresholds:    thresholds,
	}
}

func (s *SnapshotStore) ServiceLevelTarget() int {
	return s.serviceLevelTarget
}

func (s *SnapshotStore) ApplyMetadata(metas []freepbx.QueueMeta) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, m := range metas {
		q, ok := s.queues[m.Extension]
		if !ok {
			q = newQueueState(m.Extension)
			s.queues[m.Extension] = q
		}
		q.Name = m.Descr
		if m.Strategy != "" {
			q.Strategy = m.Strategy
		}
		q.Config.MaxWaitSeconds = atoiSafe(m.MaxWait)
		q.Config.Ringing = m.Ringing != 0
		q.Config.QueueWait = m.QueueWait != 0
		q.Config.MonitorType = m.MonitorType
		q.Config.ServiceLevelTargetSeconds = m.ServiceLevel
		if q.Config.ServiceLevelTargetSeconds == 0 {
			q.Config.ServiceLevelTargetSeconds = s.serviceLevelTarget
		}

		for _, member := range m.StaticMembers {
			ext := extractExtension(member)
			if ext == "" {
				continue
			}
			if _, exists := q.Members[ext]; !exists {
				q.Members[ext] = &AgentSnapshot{
					Interface:   member,
					MemberName:  member,
					Extension:   ext,
					Membership:  "dynamic",
					Status:      StatusCodeName(0),
					StatusCode:  0,
					LastEventAt: time.Now(),
				}
			}
		}
	}
	s.version++
}

func (s *SnapshotStore) MarkAMIConnected(ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.amiOK && !ok {
		s.reconnects++
	}
	s.amiOK = ok
	s.version++
}

func (s *SnapshotStore) Version() uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.version
}

// BumpIfLive incrementa version se houver dados time-dependentes vivos
// (callers esperando ou agents ringing). Sem atividade vira no-op e o
// endpoint continua respondendo 304, economizando banda.
func (s *SnapshotStore) BumpIfLive() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, q := range s.queues {
		if len(q.Callers) > 0 {
			s.version++
			return true
		}
		for _, m := range q.Members {
			if m.Ringing {
				s.version++
				return true
			}
		}
	}
	return false
}

func (s *SnapshotStore) ApplyEvent(e ami.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.applyEventLocked(e)
}

func (s *SnapshotStore) ApplyQueueStatuses(events []goami.Response) {
	s.mu.Lock()
	defer s.mu.Unlock()

	seenQueues := make(map[string]bool)
	seenCallers := make(map[string]map[string]bool)
	seenMembers := make(map[string]map[string]bool)

	for _, ev := range events {
		eventType := ev.Get("Event")
		switch eventType {
		case "QueueParams":
			queueID := ev.Get("Queue")
			if queueID == "" || queueID == "default" {
				continue
			}
			q := s.getOrCreateQueueLocked(queueID)
			if strategy := ev.Get("Strategy"); strategy != "" {
				q.Strategy = strategy
			}
			seenQueues[queueID] = true
			if seenCallers[queueID] == nil {
				seenCallers[queueID] = map[string]bool{}
			}
			if seenMembers[queueID] == nil {
				seenMembers[queueID] = map[string]bool{}
			}
		case "QueueEntry":
			queueID := ev.Get("Queue")
			if queueID == "" {
				continue
			}
			q := s.getOrCreateQueueLocked(queueID)
			uid := ev.Get("Uniqueid")
			if uid == "" {
				uid = ev.Get("Channel")
			}
			wait := atoiSafe(ev.Get("Wait"))
			c := &CallerSnapshot{
				UniqueID:     uid,
				Channel:      ev.Get("Channel"),
				CallerIDNum:  ev.Get("CallerIDNum"),
				CallerIDName: ev.Get("CallerIDName"),
				Position:     atoiSafe(ev.Get("Position")),
				EnteredAt:    time.Now().Add(-time.Duration(wait) * time.Second),
				WaitSeconds:  wait,
			}
			q.Callers[uid] = c
			if seenCallers[queueID] == nil {
				seenCallers[queueID] = map[string]bool{}
			}
			seenCallers[queueID][uid] = true
			seenQueues[queueID] = true
		case "QueueMember":
			queueID := ev.Get("Queue")
			if queueID == "" || queueID == "default" {
				continue
			}
			q := s.getOrCreateQueueLocked(queueID)
			a := buildAgentFromResponse(ev)
			if a.Extension == "" {
				continue
			}
			if existing, ok := q.Members[a.Extension]; ok {
				mergeAgentInto(existing, a)
			} else {
				q.Members[a.Extension] = a
			}
			if seenMembers[queueID] == nil {
				seenMembers[queueID] = map[string]bool{}
			}
			seenMembers[queueID][a.Extension] = true
			seenQueues[queueID] = true
		}
	}

	for qid, q := range s.queues {
		if !seenQueues[qid] {
			continue
		}
		for uid := range q.Callers {
			if !seenCallers[qid][uid] {
				delete(q.Callers, uid)
			}
		}
		for ext, m := range q.Members {
			if seenMembers[qid][ext] {
				continue
			}
			// Mantem membros estaticos vindos do FreePBX (membership=="static")
			// mesmo que o AMI nao tenha listado nessa rodada; outros somem.
			if m.Membership != "static" {
				delete(q.Members, ext)
			}
		}
	}

	s.lastEventAt = time.Now()
	s.version++
}

func (s *SnapshotStore) Snapshot() DashboardSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.buildSnapshotLocked()
}

func (s *SnapshotStore) SnapshotSince(version uint64) (bool, DashboardSnapshot) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if version > 0 && version == s.version {
		return false, DashboardSnapshot{Version: s.version}
	}
	return true, s.buildSnapshotLocked()
}

func (s *SnapshotStore) buildSnapshotLocked() DashboardSnapshot {
	now := time.Now()
	totals15m := s.metrics.Totals15m()
	totalsToday := s.metrics.TotalsToday()

	snap := DashboardSnapshot{
		Version:     s.version,
		GeneratedAt: now,
		Polling: PollingHint{
			RecommendedIntervalMs: s.pollingRecommended,
			MinIntervalMs:         s.pollingMin,
		},
		AMI: AMIStatus{
			Connected:      s.amiOK,
			LastEventAt:    s.lastEventAt,
			LastSnapshotAt: now,
			Reconnects:     s.reconnects,
		},
	}

	totals := DashboardTotals{
		Queues:         len(s.queues),
		Offered15m:     totals15m.Offered,
		Answered15m:    totals15m.Answered,
		Abandoned15m:   totals15m.Abandoned,
		AnsweredToday:  totalsToday.Answered,
		AbandonedToday: totalsToday.Abandoned,
	}
	if totals15m.Offered > 0 {
		totals.ServiceLevel15m = round2(float64(totals15m.SLAHits) / float64(totals15m.Offered) * 100.0)
		totals.AbandonRate15m = round2(float64(totals15m.Abandoned) / float64(totals15m.Offered) * 100.0)
	}
	if totals15m.Answered > 0 {
		totals.ASA15mSeconds = totals15m.HoldTimeSum / totals15m.Answered
		totals.AvgTalk15mSeconds = totals15m.TalkTimeSum / totals15m.Answered
	}

	snap.Queues = make([]QueueSnapshot, 0, len(s.queues))
	for _, q := range s.queues {
		qSnap := s.buildQueueSnapshotLocked(q, now)
		totals.Waiting += qSnap.Realtime.Waiting
		if qSnap.Realtime.LongestWaitSeconds > totals.LongestWaitSeconds {
			totals.LongestWaitSeconds = qSnap.Realtime.LongestWaitSeconds
		}
		totals.AgentsTotal += qSnap.Agents.Total
		totals.AgentsAvailable += qSnap.Agents.Available
		totals.AgentsRinging += qSnap.Agents.Ringing
		totals.AgentsInCall += qSnap.Agents.InCall
		totals.AgentsPaused += qSnap.Agents.Paused
		totals.AgentsOffline += qSnap.Agents.Offline
		snap.Queues = append(snap.Queues, qSnap)
	}
	snap.Totals = totals

	snap.Alerts = Evaluate(&snap, s.alertThresholds)
	return snap
}

func (s *SnapshotStore) buildQueueSnapshotLocked(q *queueState, now time.Time) QueueSnapshot {
	target := q.Config.ServiceLevelTargetSeconds
	if target <= 0 {
		target = s.serviceLevelTarget
	}

	callers := make([]CallerSnapshot, 0, len(q.Callers))
	longest := 0
	waitSum := 0
	for _, c := range q.Callers {
		copy := *c
		copy.WaitSeconds = int(now.Sub(c.EnteredAt).Seconds())
		if copy.WaitSeconds < 0 {
			copy.WaitSeconds = 0
		}
		copy.SLAExceeded = target > 0 && copy.WaitSeconds > target
		if copy.WaitSeconds > longest {
			longest = copy.WaitSeconds
		}
		waitSum += copy.WaitSeconds
		callers = append(callers, copy)
	}

	agents := make([]AgentSnapshot, 0, len(q.Members))
	summary := QueueAgentSummary{Total: len(q.Members)}
	for _, m := range q.Members {
		agents = append(agents, *m)
		if m.Paused {
			summary.Paused++
			continue
		}
		if m.InCall {
			summary.InCall++
			continue
		}
		if m.Ringing {
			summary.Ringing++
			continue
		}
		switch m.StatusCode {
		case 1:
			summary.Available++
		case 5, 4, 0:
			summary.Offline++
		case 2, 3, 7, 8:
			summary.InCall++
		case 6:
			summary.Ringing++
		default:
			summary.Available++
		}
	}

	q15 := s.metrics.QueueMetrics15m(q.ID, target)
	qToday := s.metrics.QueueMetricsToday(q.ID, target)

	rt := QueueRealtime{
		Waiting:            len(q.Callers),
		LongestWaitSeconds: longest,
	}
	if rt.Waiting > 0 {
		rt.AvgWaitCurrentSeconds = waitSum / rt.Waiting
	}

	return QueueSnapshot{
		ID:          q.ID,
		Name:        q.Name,
		Strategy:    q.Strategy,
		Config:      q.Config,
		Realtime:    rt,
		Agents:      summary,
		Metrics:     QueueMetricsBlock{Last15m: q15, Today: qToday},
		Callers:     callers,
		Members:     agents,
		LastEventAt: q.LastEventAt,
	}
}

func (s *SnapshotStore) getOrCreateQueueLocked(id string) *queueState {
	q, ok := s.queues[id]
	if !ok {
		q = newQueueState(id)
		s.queues[id] = q
	}
	return q
}

func newQueueState(id string) *queueState {
	return &queueState{
		ID:      id,
		Name:    id,
		Callers: make(map[string]*CallerSnapshot),
		Members: make(map[string]*AgentSnapshot),
	}
}

func buildAgentFromResponse(ev goami.Response) *AgentSnapshot {
	iface := ev.Get("Interface")
	stateIface := ev.Get("StateInterface")
	memberName := ev.Get("MemberName")

	ext := extractExtension(iface)
	if ext == "" {
		ext = extractExtension(stateIface)
	}
	if ext == "" {
		ext = extractExtension(memberName)
	}

	if iface == "" {
		if stateIface != "" {
			iface = stateIface
		} else if memberName != "" {
			iface = memberName
		} else if ext != "" {
			iface = "Local/" + ext + "@from-queue/n"
		}
	}

	statusCode := atoiSafe(ev.Get("Status"))
	paused := ev.Get("Paused") == "1"
	inCall := ev.Get("InCall") == "1"
	a := &AgentSnapshot{
		Interface:    iface,
		MemberName:   memberName,
		Extension:    ext,
		Membership:   ev.Get("Membership"),
		Status:       StatusCodeName(statusCode),
		StatusCode:   statusCode,
		Paused:       paused,
		PausedReason: ev.Get("PausedReason"),
		InCall:       inCall,
		CallsTaken:   atoiSafe(ev.Get("CallsTaken")),
		LastEventAt:  time.Now(),
	}
	if a.MemberName == "" {
		a.MemberName = a.Interface
	}
	if lastCall := ev.Get("LastCall"); lastCall != "" && lastCall != "0" {
		if ts, err := strconv.ParseInt(lastCall, 10, 64); err == nil && ts > 0 {
			t := time.Unix(ts, 0)
			a.LastCallAt = &t
		}
	}
	if statusCode == 6 {
		a.Ringing = true
	}
	return a
}

// mergeAgentInto preserva campos uteis do agente existente quando o
// QueueStatuses traz um snapshot novo: nao zera CallsTaken/LastCallAt se a
// nova resposta vier com valores menores, e nao sobrescreve Interface com
// versao menos detalhada se a antiga ja era util.
func mergeAgentInto(existing, incoming *AgentSnapshot) {
	if incoming.Interface != "" {
		existing.Interface = incoming.Interface
	}
	if incoming.MemberName != "" {
		existing.MemberName = incoming.MemberName
	}
	if incoming.Membership != "" {
		existing.Membership = incoming.Membership
	}
	existing.Status = incoming.Status
	existing.StatusCode = incoming.StatusCode
	existing.Paused = incoming.Paused
	if incoming.PausedReason != "" {
		existing.PausedReason = incoming.PausedReason
	} else if !existing.Paused {
		existing.PausedReason = ""
	}
	existing.InCall = incoming.InCall
	existing.Ringing = incoming.Ringing
	if incoming.CallsTaken > existing.CallsTaken {
		existing.CallsTaken = incoming.CallsTaken
	}
	if incoming.LastCallAt != nil {
		existing.LastCallAt = incoming.LastCallAt
	}
	existing.LastEventAt = incoming.LastEventAt
}

func atoiSafe(v string) int {
	if v == "" {
		return 0
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0
	}
	return n
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}

func extractExtension(iface string) string {
	if iface == "" {
		return ""
	}
	prefixes := []string{"Local/", "PJSIP/", "SIP/", "IAX2/", "DAHDI/"}
	rest := iface
	for _, p := range prefixes {
		if len(rest) > len(p) && rest[:len(p)] == p {
			rest = rest[len(p):]
			break
		}
	}
	for i := 0; i < len(rest); i++ {
		ch := rest[i]
		if ch == '@' || ch == '-' || ch == '/' {
			return rest[:i]
		}
	}
	return rest
}

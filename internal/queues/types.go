package queues

import "time"

type DashboardSnapshot struct {
	Version     uint64           `json:"version"`
	GeneratedAt time.Time        `json:"generated_at"`
	Stale       bool             `json:"stale,omitempty"`
	Polling     PollingHint      `json:"polling"`
	AMI         AMIStatus        `json:"ami"`
	Totals      DashboardTotals  `json:"totals"`
	Queues      []QueueSnapshot  `json:"queues"`
	Alerts      []DashboardAlert `json:"alerts"`
}

type PollingHint struct {
	RecommendedIntervalMs int `json:"recommended_interval_ms"`
	MinIntervalMs         int `json:"min_interval_ms"`
}

type AMIStatus struct {
	Connected      bool      `json:"connected"`
	LastEventAt    time.Time `json:"last_event_at"`
	LastSnapshotAt time.Time `json:"last_snapshot_at"`
	Reconnects     int       `json:"reconnects"`
}

type DashboardTotals struct {
	Queues             int     `json:"queues"`
	Waiting            int     `json:"waiting"`
	LongestWaitSeconds int     `json:"longest_wait_seconds"`
	AgentsTotal        int     `json:"agents_total"`
	AgentsAvailable    int     `json:"agents_available"`
	AgentsRinging      int     `json:"agents_ringing"`
	AgentsInCall       int     `json:"agents_in_call"`
	AgentsPaused       int     `json:"agents_paused"`
	AgentsOffline      int     `json:"agents_offline"`
	Offered15m         int     `json:"offered_15m"`
	Answered15m        int     `json:"answered_15m"`
	Abandoned15m       int     `json:"abandoned_15m"`
	ServiceLevel15m    float64 `json:"service_level_15m"`
	AbandonRate15m     float64 `json:"abandon_rate_15m"`
	ASA15mSeconds      int     `json:"asa_15m_seconds"`
	AvgTalk15mSeconds  int     `json:"avg_talk_15m_seconds"`
	AnsweredToday      int     `json:"answered_today"`
	AbandonedToday     int     `json:"abandoned_today"`
}

type QueueSnapshot struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Strategy    string            `json:"strategy"`
	Config      QueueConfigBlock  `json:"config"`
	Realtime    QueueRealtime     `json:"realtime"`
	Agents      QueueAgentSummary `json:"agents"`
	Metrics     QueueMetricsBlock `json:"metrics"`
	Callers     []CallerSnapshot  `json:"callers"`
	Members     []AgentSnapshot   `json:"members"`
	Alerts      []DashboardAlert  `json:"alerts"`
	LastEventAt time.Time         `json:"last_event_at"`
}

type QueueConfigBlock struct {
	MaxWaitSeconds            int    `json:"max_wait_seconds"`
	ServiceLevelTargetSeconds int    `json:"service_level_target_seconds"`
	Ringing                   bool   `json:"ringing"`
	QueueWait                 bool   `json:"queue_wait"`
	MonitorType               string `json:"monitor_type"`
}

type QueueRealtime struct {
	Waiting               int `json:"waiting"`
	LongestWaitSeconds    int `json:"longest_wait_seconds"`
	AvgWaitCurrentSeconds int `json:"avg_wait_current_seconds"`
}

type QueueAgentSummary struct {
	Total     int `json:"total"`
	Available int `json:"available"`
	Ringing   int `json:"ringing"`
	InCall    int `json:"in_call"`
	Paused    int `json:"paused"`
	Offline   int `json:"offline"`
}

type QueueMetricsBlock struct {
	Last15m QueueMetrics `json:"last_15m"`
	Today   QueueMetrics `json:"today"`
}

type QueueMetrics struct {
	Offered        int     `json:"offered"`
	Answered       int     `json:"answered"`
	Abandoned      int     `json:"abandoned"`
	ServiceLevel   float64 `json:"service_level"`
	AbandonRate    float64 `json:"abandon_rate"`
	ASASeconds     int     `json:"asa_seconds"`
	AvgTalkSeconds int     `json:"avg_talk_seconds"`
	MaxWaitSeconds int     `json:"max_wait_seconds"`
}

type CallerSnapshot struct {
	UniqueID     string    `json:"uniqueid"`
	LinkedID     string    `json:"linkedid"`
	Channel      string    `json:"channel"`
	CallerIDNum  string    `json:"callerid_num"`
	CallerIDName string    `json:"callerid_name"`
	Position     int       `json:"position"`
	EnteredAt    time.Time `json:"entered_at"`
	WaitSeconds  int       `json:"wait_seconds"`
	SLAExceeded  bool      `json:"sla_exceeded"`
}

type AgentSnapshot struct {
	Interface    string     `json:"interface"`
	MemberName   string     `json:"member_name"`
	Extension    string     `json:"extension"`
	Membership   string     `json:"membership"`
	Status       string     `json:"status"`
	StatusCode   int        `json:"status_code"`
	Paused       bool       `json:"paused"`
	PausedReason string     `json:"paused_reason,omitempty"`
	InCall       bool       `json:"in_call"`
	Ringing      bool       `json:"ringing"`
	CallsTaken   int        `json:"calls_taken"`
	LastCallAt   *time.Time `json:"last_call_at,omitempty"`
	LastEventAt  time.Time  `json:"last_event_at"`
}

type DashboardAlert struct {
	Level     string    `json:"level"`
	Code      string    `json:"code"`
	Queue     string    `json:"queue,omitempty"`
	Message   string    `json:"message"`
	Value     float64   `json:"value,omitempty"`
	Threshold float64   `json:"threshold,omitempty"`
	StartedAt time.Time `json:"started_at"`
}

func StatusCodeName(code int) string {
	switch code {
	case 0:
		return "UNKNOWN"
	case 1:
		return "NOT_INUSE"
	case 2:
		return "INUSE"
	case 3:
		return "BUSY"
	case 4:
		return "INVALID"
	case 5:
		return "UNAVAILABLE"
	case 6:
		return "RINGING"
	case 7:
		return "RINGINUSE"
	case 8:
		return "ONHOLD"
	default:
		return "UNKNOWN"
	}
}

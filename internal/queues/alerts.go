package queues

import (
	"fmt"
	"time"
)

type AlertThresholds struct {
	WarningLongestWaitSeconds  int     `json:"warning_longest_wait_seconds"`
	CriticalLongestWaitSeconds int     `json:"critical_longest_wait_seconds"`
	WarningWaiting             int     `json:"warning_waiting"`
	CriticalWaiting            int     `json:"critical_waiting"`
	WarningServiceLevel        float64 `json:"warning_service_level"`
	CriticalServiceLevel       float64 `json:"critical_service_level"`
	WarningAbandonRate         float64 `json:"warning_abandon_rate"`
	CriticalAbandonRate        float64 `json:"critical_abandon_rate"`
	WarningASASeconds          int     `json:"warning_asa_seconds"`
	CriticalASASeconds         int     `json:"critical_asa_seconds"`
	WarningPausedAgentsPercent float64 `json:"warning_paused_agents_percent"`
	CriticalPausedAgentsPercent float64 `json:"critical_paused_agents_percent"`
}

func DefaultThresholds() AlertThresholds {
	return AlertThresholds{
		WarningLongestWaitSeconds:   30,
		CriticalLongestWaitSeconds:  60,
		WarningWaiting:              3,
		CriticalWaiting:             6,
		WarningServiceLevel:         80,
		CriticalServiceLevel:        70,
		WarningAbandonRate:          3,
		CriticalAbandonRate:         5,
		WarningASASeconds:           30,
		CriticalASASeconds:          60,
		WarningPausedAgentsPercent:  30,
		CriticalPausedAgentsPercent: 50,
	}
}

func Evaluate(snap *DashboardSnapshot, t AlertThresholds) []DashboardAlert {
	if t == (AlertThresholds{}) {
		t = DefaultThresholds()
	}
	now := time.Now()
	var alerts []DashboardAlert

	for i := range snap.Queues {
		q := &snap.Queues[i]
		qAlerts := evaluateQueue(q, t, now)
		snap.Queues[i].Alerts = qAlerts
		alerts = append(alerts, qAlerts...)
	}

	return alerts
}

func evaluateQueue(q *QueueSnapshot, t AlertThresholds, now time.Time) []DashboardAlert {
	var out []DashboardAlert

	if t.CriticalLongestWaitSeconds > 0 && q.Realtime.LongestWaitSeconds >= t.CriticalLongestWaitSeconds {
		out = append(out, DashboardAlert{
			Level:     "critical",
			Code:      "LONGEST_WAIT",
			Queue:     q.ID,
			Message:   fmt.Sprintf("espera m\u00e1xima da fila %s >= %ds", q.ID, t.CriticalLongestWaitSeconds),
			Value:     float64(q.Realtime.LongestWaitSeconds),
			Threshold: float64(t.CriticalLongestWaitSeconds),
			StartedAt: now,
		})
	} else if t.WarningLongestWaitSeconds > 0 && q.Realtime.LongestWaitSeconds >= t.WarningLongestWaitSeconds {
		out = append(out, DashboardAlert{
			Level:     "warning",
			Code:      "LONGEST_WAIT",
			Queue:     q.ID,
			Message:   fmt.Sprintf("espera m\u00e1xima da fila %s >= %ds", q.ID, t.WarningLongestWaitSeconds),
			Value:     float64(q.Realtime.LongestWaitSeconds),
			Threshold: float64(t.WarningLongestWaitSeconds),
			StartedAt: now,
		})
	}

	if t.CriticalWaiting > 0 && q.Realtime.Waiting >= t.CriticalWaiting {
		out = append(out, DashboardAlert{
			Level:     "critical",
			Code:      "WAITING",
			Queue:     q.ID,
			Message:   fmt.Sprintf("%d aguardando na fila %s", q.Realtime.Waiting, q.ID),
			Value:     float64(q.Realtime.Waiting),
			Threshold: float64(t.CriticalWaiting),
			StartedAt: now,
		})
	} else if t.WarningWaiting > 0 && q.Realtime.Waiting >= t.WarningWaiting {
		out = append(out, DashboardAlert{
			Level:     "warning",
			Code:      "WAITING",
			Queue:     q.ID,
			Message:   fmt.Sprintf("%d aguardando na fila %s", q.Realtime.Waiting, q.ID),
			Value:     float64(q.Realtime.Waiting),
			Threshold: float64(t.WarningWaiting),
			StartedAt: now,
		})
	}

	sl := q.Metrics.Last15m.ServiceLevel
	if q.Metrics.Last15m.Offered > 0 {
		if t.CriticalServiceLevel > 0 && sl < t.CriticalServiceLevel {
			out = append(out, DashboardAlert{
				Level:     "critical",
				Code:      "SLA_LOW",
				Queue:     q.ID,
				Message:   fmt.Sprintf("SLA da fila %s abaixo de %.0f%% nos \u00faltimos 15min", q.ID, t.CriticalServiceLevel),
				Value:     sl,
				Threshold: t.CriticalServiceLevel,
				StartedAt: now,
			})
		} else if t.WarningServiceLevel > 0 && sl < t.WarningServiceLevel {
			out = append(out, DashboardAlert{
				Level:     "warning",
				Code:      "SLA_LOW",
				Queue:     q.ID,
				Message:   fmt.Sprintf("SLA da fila %s abaixo de %.0f%% nos \u00faltimos 15min", q.ID, t.WarningServiceLevel),
				Value:     sl,
				Threshold: t.WarningServiceLevel,
				StartedAt: now,
			})
		}
	}

	ab := q.Metrics.Last15m.AbandonRate
	if q.Metrics.Last15m.Offered > 0 {
		if t.CriticalAbandonRate > 0 && ab > t.CriticalAbandonRate {
			out = append(out, DashboardAlert{
				Level:     "critical",
				Code:      "ABANDON_RATE",
				Queue:     q.ID,
				Message:   fmt.Sprintf("taxa de abandono da fila %s acima de %.0f%%", q.ID, t.CriticalAbandonRate),
				Value:     ab,
				Threshold: t.CriticalAbandonRate,
				StartedAt: now,
			})
		} else if t.WarningAbandonRate > 0 && ab > t.WarningAbandonRate {
			out = append(out, DashboardAlert{
				Level:     "warning",
				Code:      "ABANDON_RATE",
				Queue:     q.ID,
				Message:   fmt.Sprintf("taxa de abandono da fila %s acima de %.0f%%", q.ID, t.WarningAbandonRate),
				Value:     ab,
				Threshold: t.WarningAbandonRate,
				StartedAt: now,
			})
		}
	}

	asa := q.Metrics.Last15m.ASASeconds
	if q.Metrics.Last15m.Answered > 0 {
		if t.CriticalASASeconds > 0 && asa > t.CriticalASASeconds {
			out = append(out, DashboardAlert{
				Level:     "critical",
				Code:      "ASA",
				Queue:     q.ID,
				Message:   fmt.Sprintf("ASA da fila %s > %ds", q.ID, t.CriticalASASeconds),
				Value:     float64(asa),
				Threshold: float64(t.CriticalASASeconds),
				StartedAt: now,
			})
		} else if t.WarningASASeconds > 0 && asa > t.WarningASASeconds {
			out = append(out, DashboardAlert{
				Level:     "warning",
				Code:      "ASA",
				Queue:     q.ID,
				Message:   fmt.Sprintf("ASA da fila %s > %ds", q.ID, t.WarningASASeconds),
				Value:     float64(asa),
				Threshold: float64(t.WarningASASeconds),
				StartedAt: now,
			})
		}
	}

	if q.Agents.Total > 0 {
		pct := float64(q.Agents.Paused) / float64(q.Agents.Total) * 100.0
		if t.CriticalPausedAgentsPercent > 0 && pct > t.CriticalPausedAgentsPercent {
			out = append(out, DashboardAlert{
				Level:     "critical",
				Code:      "PAUSED_AGENTS",
				Queue:     q.ID,
				Message:   fmt.Sprintf("%.0f%% dos agentes da fila %s em pausa", pct, q.ID),
				Value:     round2(pct),
				Threshold: t.CriticalPausedAgentsPercent,
				StartedAt: now,
			})
		} else if t.WarningPausedAgentsPercent > 0 && pct > t.WarningPausedAgentsPercent {
			out = append(out, DashboardAlert{
				Level:     "warning",
				Code:      "PAUSED_AGENTS",
				Queue:     q.ID,
				Message:   fmt.Sprintf("%.0f%% dos agentes da fila %s em pausa", pct, q.ID),
				Value:     round2(pct),
				Threshold: t.WarningPausedAgentsPercent,
				StartedAt: now,
			})
		}
	}

	return out
}

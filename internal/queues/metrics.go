package queues

import (
	"sync"
	"time"
)

type RollingEventType string

const (
	EventOffered   RollingEventType = "offered"
	EventAnswered  RollingEventType = "answered"
	EventAbandoned RollingEventType = "abandoned"
)

type RollingEvent struct {
	At       time.Time
	Queue    string
	Type     RollingEventType
	HoldTime int
	TalkTime int
	SLATier  int
}

type aggregate struct {
	Offered     int
	Answered    int
	Abandoned   int
	HoldTimeSum int
	TalkTimeSum int
	SLAHits     int
	MaxWait     int
}

type RollingMetrics struct {
	mu     sync.Mutex
	events []RollingEvent
	today  map[string]*aggregate
	todayD time.Time
}

func NewRollingMetrics() *RollingMetrics {
	return &RollingMetrics{
		today:  make(map[string]*aggregate),
		todayD: startOfDay(time.Now()),
	}
}

func (m *RollingMetrics) Record(ev RollingEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.rotateDayLocked(ev.At)
	m.events = append(m.events, ev)
	m.pruneLocked(ev.At)

	agg, ok := m.today[ev.Queue]
	if !ok {
		agg = &aggregate{}
		m.today[ev.Queue] = agg
	}
	applyEventToAggregate(agg, ev)
}

func (m *RollingMetrics) AddTalkTimeToLastAnswered(queue string, talkTime int) {
	if talkTime <= 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := len(m.events) - 1; i >= 0; i-- {
		if m.events[i].Queue == queue && m.events[i].Type == EventAnswered && m.events[i].TalkTime == 0 {
			m.events[i].TalkTime = talkTime
			if agg := m.today[queue]; agg != nil {
				agg.TalkTimeSum += talkTime
			}
			return
		}
	}
}

func (m *RollingMetrics) Totals15m() aggregate {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pruneLocked(time.Now())

	out := aggregate{}
	for i := range m.events {
		applyEventToAggregate(&out, m.events[i])
	}
	return out
}

func (m *RollingMetrics) TotalsToday() aggregate {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rotateDayLocked(time.Now())

	out := aggregate{}
	for _, agg := range m.today {
		out.Offered += agg.Offered
		out.Answered += agg.Answered
		out.Abandoned += agg.Abandoned
		out.HoldTimeSum += agg.HoldTimeSum
		out.TalkTimeSum += agg.TalkTimeSum
		out.SLAHits += agg.SLAHits
		if agg.MaxWait > out.MaxWait {
			out.MaxWait = agg.MaxWait
		}
	}
	return out
}

func (m *RollingMetrics) QueueMetrics15m(queue string, target int) QueueMetrics {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pruneLocked(time.Now())

	agg := aggregate{}
	for i := range m.events {
		if m.events[i].Queue == queue {
			applyEventToAggregate(&agg, m.events[i])
		}
	}
	return toQueueMetrics(agg)
}

func (m *RollingMetrics) QueueMetricsToday(queue string, target int) QueueMetrics {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rotateDayLocked(time.Now())

	agg := m.today[queue]
	if agg == nil {
		return QueueMetrics{}
	}
	return toQueueMetrics(*agg)
}

func (m *RollingMetrics) pruneLocked(now time.Time) {
	cutoff := now.Add(-15 * time.Minute)
	idx := 0
	for idx < len(m.events) && m.events[idx].At.Before(cutoff) {
		idx++
	}
	if idx > 0 {
		m.events = m.events[idx:]
	}
}

func (m *RollingMetrics) rotateDayLocked(t time.Time) {
	day := startOfDay(t)
	if !day.Equal(m.todayD) {
		m.today = make(map[string]*aggregate)
		m.todayD = day
	}
}

func applyEventToAggregate(agg *aggregate, ev RollingEvent) {
	switch ev.Type {
	case EventOffered:
		agg.Offered++
		if ev.HoldTime > agg.MaxWait {
			agg.MaxWait = ev.HoldTime
		}
	case EventAnswered:
		agg.Answered++
		agg.HoldTimeSum += ev.HoldTime
		agg.TalkTimeSum += ev.TalkTime
		if ev.SLATier == 1 {
			agg.SLAHits++
		}
		if ev.HoldTime > agg.MaxWait {
			agg.MaxWait = ev.HoldTime
		}
	case EventAbandoned:
		agg.Abandoned++
		if ev.HoldTime > agg.MaxWait {
			agg.MaxWait = ev.HoldTime
		}
	}
}

func toQueueMetrics(agg aggregate) QueueMetrics {
	out := QueueMetrics{
		Offered:        agg.Offered,
		Answered:       agg.Answered,
		Abandoned:      agg.Abandoned,
		MaxWaitSeconds: agg.MaxWait,
	}
	if agg.Offered > 0 {
		out.ServiceLevel = round2(float64(agg.SLAHits) / float64(agg.Offered) * 100.0)
		out.AbandonRate = round2(float64(agg.Abandoned) / float64(agg.Offered) * 100.0)
	}
	if agg.Answered > 0 {
		out.ASASeconds = agg.HoldTimeSum / agg.Answered
		out.AvgTalkSeconds = agg.TalkTimeSum / agg.Answered
	}
	return out
}

func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

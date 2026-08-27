package queues

import (
	"log"
	"time"

	"github.com/safehouse/amigow/internal/ami"
)

func RunReducer(ch <-chan ami.Event, store *SnapshotStore, eventsPath string, dirty chan<- struct{}) {
	log.Println("[DASH] reducer iniciado")
	for ev := range ch {
		before := store.Version()
		store.ApplyEvent(ev)
		after := store.Version()
		if after != before {
			log.Printf("[REDUCER] %s queue=%s version=%d", ev.Type, ev.Data["Queue"], after)
			if dirty != nil {
				select {
				case dirty <- struct{}{}:
				default:
				}
			}
		}
		if eventsPath != "" && relevantForPersist(ev.Type) {
			AppendEvent(eventsPath, ev)
		}
	}
	log.Println("[DASH] reducer encerrado")
}

func relevantForPersist(t string) bool {
	switch t {
	case "queue_join", "queue_abandon", "agent_connect", "agent_ring_no_answer", "agent_called", "agent_complete", "queue_member_status", "queue_member_pause", "queue_member_added", "queue_member_removed":
		return true
	}
	return false
}

func (s *SnapshotStore) applyEventLocked(e ami.Event) {
	queueID := e.Data["Queue"]
	if queueID == "default" {
		return
	}

	if queueID == "" && requiresQueue(e.Type) {
		if q := s.lookupQueueForCallerLocked(e); q != nil {
			queueID = q.ID
		} else {
			return
		}
	}

	var q *queueState
	if queueID != "" {
		q = s.getOrCreateQueueLocked(queueID)
		q.LastEventAt = e.Timestamp
	}
	s.lastEventAt = e.Timestamp

	switch e.Type {
	case "queue_join":
		s.applyQueueJoinLocked(q, e)
	case "queue_leave":
		s.applyQueueLeaveLocked(q, e)
	case "queue_abandon":
		s.applyQueueAbandonLocked(q, e)
	case "queue_params":
		if strategy := e.Data["Strategy"]; strategy != "" {
			q.Strategy = strategy
		}
	case "agent_called":
		s.applyAgentCalledLocked(q, e)
	case "agent_ring_no_answer":
		s.applyAgentRingNoAnswerLocked(q, e)
	case "agent_connect":
		s.applyAgentConnectLocked(q, e)
	case "agent_complete":
		s.applyAgentCompleteLocked(q, e)
	case "agent_dump":
		s.applyAgentRingNoAnswerLocked(q, e)
	case "queue_member_status", "queue_member_pause", "queue_member_added":
		s.applyMemberUpsertLocked(q, e)
	case "queue_member_removed":
		s.applyMemberRemovedLocked(q, e)
	}

	s.version++
}

func requiresQueue(eventType string) bool {
	switch eventType {
	case "queue_join", "queue_leave", "queue_abandon", "queue_params",
		"agent_called", "agent_ring_no_answer", "agent_connect", "agent_complete", "agent_dump",
		"queue_member_status", "queue_member_pause", "queue_member_added", "queue_member_removed":
		return true
	}
	return false
}

// lookupQueueForCallerLocked tenta achar a queue dona de um caller quando o
// evento chega sem o campo Queue (cenario visto em alguns QueueCallerAbandon
// e AgentComplete). Procura por Uniqueid e por Linkedid.
func (s *SnapshotStore) lookupQueueForCallerLocked(e ami.Event) *queueState {
	uid := e.Data["Uniqueid"]
	linkedID := e.Data["Linkedid"]
	if uid == "" && linkedID == "" {
		return nil
	}
	for _, q := range s.queues {
		if uid != "" {
			if _, ok := q.Callers[uid]; ok {
				return q
			}
		}
		if linkedID != "" {
			for _, c := range q.Callers {
				if c.LinkedID == linkedID {
					return q
				}
			}
		}
	}
	return nil
}

// memberExtension extrai a extensao do membro a partir de um evento AMI,
// com fallback porque Interface as vezes vem vazio (caso real:
// QueueMemberRemoved traz apenas StateInterface/MemberName).
func memberExtension(e ami.Event) string {
	if ext := extractExtension(e.Data["Interface"]); ext != "" {
		return ext
	}
	if ext := extractExtension(e.Data["StateInterface"]); ext != "" {
		return ext
	}
	if ext := extractExtension(e.Data["MemberName"]); ext != "" {
		return ext
	}
	return ""
}

// memberInterface devolve a melhor representacao de Interface conhecida no
// evento (Interface > StateInterface > MemberName > Local/<ext>@from-queue/n).
func memberInterface(e ami.Event, ext string) string {
	if v := e.Data["Interface"]; v != "" {
		return v
	}
	if v := e.Data["StateInterface"]; v != "" {
		return v
	}
	if v := e.Data["MemberName"]; v != "" {
		return v
	}
	if ext != "" {
		return "Local/" + ext + "@from-queue/n"
	}
	return ""
}

func (s *SnapshotStore) applyQueueJoinLocked(q *queueState, e ami.Event) {
	uid := e.Data["Uniqueid"]
	if uid == "" {
		uid = e.Data["Channel"]
	}
	c := &CallerSnapshot{
		UniqueID:     uid,
		LinkedID:     e.Data["Linkedid"],
		Channel:      e.Data["Channel"],
		CallerIDNum:  e.Data["CallerIDNum"],
		CallerIDName: e.Data["CallerIDName"],
		Position:     atoiSafe(e.Data["Position"]),
		EnteredAt:    e.Timestamp,
	}
	q.Callers[uid] = c
	s.metrics.Record(RollingEvent{
		At:    e.Timestamp,
		Queue: q.ID,
		Type:  EventOffered,
	})
}

func (s *SnapshotStore) applyQueueLeaveLocked(q *queueState, e ami.Event) {
	uid := e.Data["Uniqueid"]
	if uid == "" {
		uid = e.Data["Channel"]
	}
	if uid != "" {
		delete(q.Callers, uid)
		return
	}
	if linkedID := e.Data["Linkedid"]; linkedID != "" {
		for k, c := range q.Callers {
			if c.LinkedID == linkedID {
				delete(q.Callers, k)
				return
			}
		}
	}
}

func (s *SnapshotStore) applyQueueAbandonLocked(q *queueState, e ami.Event) {
	uid := e.Data["Uniqueid"]
	if uid == "" {
		uid = e.Data["Channel"]
	}
	removed := false
	if uid != "" {
		if _, ok := q.Callers[uid]; ok {
			delete(q.Callers, uid)
			removed = true
		}
	}
	if !removed {
		if linkedID := e.Data["Linkedid"]; linkedID != "" {
			for k, c := range q.Callers {
				if c.LinkedID == linkedID {
					delete(q.Callers, k)
					break
				}
			}
		}
	}
	hold := atoiSafe(e.Data["HoldTime"])
	s.metrics.Record(RollingEvent{
		At:       e.Timestamp,
		Queue:    q.ID,
		Type:     EventAbandoned,
		HoldTime: hold,
	})
}

func (s *SnapshotStore) applyAgentCalledLocked(q *queueState, e ami.Event) {
	ext := memberExtension(e)
	if ext == "" {
		return
	}
	a := s.ensureMemberLocked(q, ext, e)
	a.Ringing = true
	a.LastEventAt = e.Timestamp
}

func (s *SnapshotStore) applyAgentRingNoAnswerLocked(q *queueState, e ami.Event) {
	ext := memberExtension(e)
	if ext == "" {
		return
	}
	a := s.ensureMemberLocked(q, ext, e)
	a.Ringing = false
	a.LastEventAt = e.Timestamp
}

func (s *SnapshotStore) applyAgentConnectLocked(q *queueState, e ami.Event) {
	uid := e.Data["Uniqueid"]
	if uid == "" {
		uid = e.Data["Channel"]
	}
	if uid != "" {
		delete(q.Callers, uid)
	} else if linkedID := e.Data["Linkedid"]; linkedID != "" {
		for k, c := range q.Callers {
			if c.LinkedID == linkedID {
				delete(q.Callers, k)
				break
			}
		}
	}

	if ext := memberExtension(e); ext != "" {
		a := s.ensureMemberLocked(q, ext, e)
		a.InCall = true
		a.Ringing = false
		a.CallsTaken++
		now := e.Timestamp
		a.LastCallAt = &now
		a.LastEventAt = e.Timestamp
	}

	hold := atoiSafe(e.Data["HoldTime"])
	tier := 0
	target := q.Config.ServiceLevelTargetSeconds
	if target <= 0 {
		target = s.serviceLevelTarget
	}
	if target > 0 && hold <= target {
		tier = 1
	}
	s.metrics.Record(RollingEvent{
		At:       e.Timestamp,
		Queue:    q.ID,
		Type:     EventAnswered,
		HoldTime: hold,
		SLATier:  tier,
	})
}

func (s *SnapshotStore) applyAgentCompleteLocked(q *queueState, e ami.Event) {
	ext := memberExtension(e)
	if ext == "" {
		return
	}
	a := s.ensureMemberLocked(q, ext, e)
	a.InCall = false
	a.LastEventAt = e.Timestamp
	talk := atoiSafe(e.Data["TalkTime"])
	if talk > 0 {
		s.metrics.AddTalkTimeToLastAnswered(q.ID, talk)
	}
}

func (s *SnapshotStore) applyMemberUpsertLocked(q *queueState, e ami.Event) {
	log.Printf("[REDUCER] applyMemberUpsertLocked: %v", e)
	ext := memberExtension(e)
	if ext == "" {
		return
	}
	a := s.ensureMemberLocked(q, ext, e)
	if name := e.Data["MemberName"]; name != "" {
		a.MemberName = name
	}
	if mem := e.Data["Membership"]; mem != "" {
		a.Membership = mem
	}
	if status := e.Data["Status"]; status != "" {
		a.StatusCode = atoiSafe(status)
		a.Status = StatusCodeName(a.StatusCode)
	}
	if paused := e.Data["Paused"]; paused != "" {
		a.Paused = paused == "1"
	}
	if reason := e.Data["PausedReason"]; reason != "" {
		a.PausedReason = reason
	}
	if inCall := e.Data["InCall"]; inCall != "" {
		a.InCall = inCall == "1"
	}
	if calls := e.Data["CallsTaken"]; calls != "" {
		a.CallsTaken = atoiSafe(calls)
	}
	if a.StatusCode == 6 {
		a.Ringing = true
	} else if a.StatusCode == 1 {
		a.Ringing = false
	}
	a.LastEventAt = e.Timestamp
}

func (s *SnapshotStore) applyMemberRemovedLocked(q *queueState, e ami.Event) {
	ext := memberExtension(e)
	if ext == "" {
		return
	}
	delete(q.Members, ext)
}

func (s *SnapshotStore) ensureMemberLocked(q *queueState, ext string, e ami.Event) *AgentSnapshot {
	a, ok := q.Members[ext]
	if !ok {
		a = &AgentSnapshot{
			Interface:   memberInterface(e, ext),
			MemberName:  e.Data["MemberName"],
			Extension:   ext,
			Membership:  e.Data["Membership"],
			Status:      StatusCodeName(0),
			StatusCode:  0,
			LastEventAt: time.Now(),
		}
		if a.MemberName == "" {
			a.MemberName = a.Interface
		}
		q.Members[ext] = a
	} else {
		// Prefere Interface canonica (Local/...) ao update; mantem se ja temos
		if a.Interface == "" {
			a.Interface = memberInterface(e, ext)
		}
	}
	return a
}

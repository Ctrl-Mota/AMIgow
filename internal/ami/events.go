package ami

import (
	"time"

	goami "github.com/heltonmarx/goami/ami"
)

type Event struct {
	Type      string
	Source    string
	Timestamp time.Time
	Data      map[string]string
}

func ProcessAMIEvent(amiEvent goami.Response, sourceID string) (*Event, bool) {
	eventType := amiEvent.Get("Event")

	if eventType == "" {
		return nil, false
	}
	// log.Printf("[AMI] Eveneto: %s - %v", eventType, amiEvent)

	if isAnswerEvent(amiEvent) {
		return buildEvent("answer", sourceID, amiEvent), true
	}

	if isHangupEvent(amiEvent) {
		return buildEvent("hangup", sourceID, amiEvent), true
	}

	if isMissedCallEvent(amiEvent) {
		return buildEvent("missed", sourceID, amiEvent), true
	}

	if isNewChannelEvent(amiEvent) {
		return buildEvent("invite", sourceID, amiEvent), true
	}

	if isQueueCallerJoinEvent(amiEvent) {
		return buildEvent("queue_join", sourceID, amiEvent), true
	}

	if isQueueCallerLeaveEvent(amiEvent) {
		return buildEvent("queue_leave", sourceID, amiEvent), true
	}

	if isQueueCallerAbandonEvent(amiEvent) {
		return buildEvent("queue_abandon", sourceID, amiEvent), true
	}
	if isQueueParamsEvent(amiEvent) {
		return buildEvent("queue_params", sourceID, amiEvent), true
	}

	if isQueueEntryEvent(amiEvent) {
		return buildEvent("queue_entry", sourceID, amiEvent), true
	}
	if isQueueMemberEvent(amiEvent) {
		return buildEvent("queue_member", sourceID, amiEvent), true
	}
	if isQueueMemberStatusEvent(amiEvent) {
		return buildEvent("queue_member_status", sourceID, amiEvent), true
	}
	if isQueueMemberPauseEvent(amiEvent) {
		return buildEvent("queue_member_pause", sourceID, amiEvent), true
	}
	if isQueueMemberAddedEvent(amiEvent) {
		return buildEvent("queue_member_added", sourceID, amiEvent), true
	}
	if isQueueMemberRemovedEvent(amiEvent) {
		return buildEvent("queue_member_removed", sourceID, amiEvent), true
	}
	if isQueueStatusCompleteEvent(amiEvent) {
		return buildEvent("queue_status_complete", sourceID, amiEvent), true
	}

	if isAgentCalledEvent(amiEvent) {
		return buildEvent("agent_called", sourceID, amiEvent), true
	}
	if isAgentCompleteEvent(amiEvent) {
		return buildEvent("agent_complete", sourceID, amiEvent), true
	}
	if isAgentConnectEvent(amiEvent) {
		return buildEvent("agent_connect", sourceID, amiEvent), true
	}
	if isAgentDumpEvent(amiEvent) {
		return buildEvent("agent_dump", sourceID, amiEvent), true
	}
	if isAgentRingNoAnswerEvent(amiEvent) {
		return buildEvent("agent_ring_no_answer", sourceID, amiEvent), true
	}

	if isDTMFBeginEvent(amiEvent) {
		return buildEvent("dtmf_begin", sourceID, amiEvent), true
	}
	if isDTMFEndEvent(amiEvent) {
		return buildEvent("dtmf_end", sourceID, amiEvent), true
	}

	return nil, false
}

func isAnswerEvent(amiEvent goami.Response) bool {
	eventType := amiEvent.Get("Event")

	if eventType == "Newchannel" {
		channelState := amiEvent.Get("ChannelStateDesc")
		if channelState == "Up" {
			return true
		}
	}

	if eventType == "Bridge" {
		bridgeState := amiEvent.Get("BridgeState")
		if bridgeState == "Link" {
			return true
		}
	}

	return false
}

func isQueueParamsEvent(amiEvent goami.Response) bool {
	return amiEvent.Get("Event") == "QueueParams"
}

func isHangupEvent(amiEvent goami.Response) bool {
	return amiEvent.Get("Event") == "Hangup"
}

func isMissedCallEvent(amiEvent goami.Response) bool {
	eventType := amiEvent.Get("Event")

	if eventType == "DialEnd" {
		dialStatus := amiEvent.Get("DialStatus")
		if dialStatus == "NOANSWER" || dialStatus == "BUSY" || dialStatus == "CANCEL" {
			return true
		}
	}

	if eventType == "Hangup" {
		cause := amiEvent.Get("Cause")
		if cause == "19" || cause == "17" || cause == "18" || cause == "21" {
			return true
		}
	}

	return false
}

func isQueueCallerJoinEvent(amiEvent goami.Response) bool {
	return amiEvent.Get("Event") == "QueueCallerJoin"
}

func isQueueCallerLeaveEvent(amiEvent goami.Response) bool {
	return amiEvent.Get("Event") == "QueueCallerLeave"
}

func isQueueCallerAbandonEvent(amiEvent goami.Response) bool {
	return amiEvent.Get("Event") == "QueueCallerAbandon"
}

func isQueueEntryEvent(amiEvent goami.Response) bool {
	return amiEvent.Get("Event") == "QueueEntry"
}

func isQueueMemberEvent(amiEvent goami.Response) bool {
	return amiEvent.Get("Event") == "QueueMember"
}

func isQueueMemberStatusEvent(amiEvent goami.Response) bool {
	return amiEvent.Get("Event") == "QueueMemberStatus"
}

func isQueueMemberPauseEvent(amiEvent goami.Response) bool {
	return amiEvent.Get("Event") == "QueueMemberPause"
}

func isQueueMemberAddedEvent(amiEvent goami.Response) bool {
	return amiEvent.Get("Event") == "QueueMemberAdded"
}

func isQueueMemberRemovedEvent(amiEvent goami.Response) bool {
	return amiEvent.Get("Event") == "QueueMemberRemoved"
}

func isQueueStatusCompleteEvent(amiEvent goami.Response) bool {
	return amiEvent.Get("Event") == "QueueStatusComplete"
}

func isAgentCalledEvent(amiEvent goami.Response) bool {
	return amiEvent.Get("Event") == "AgentCalled"
}

func isAgentCompleteEvent(amiEvent goami.Response) bool {
	return amiEvent.Get("Event") == "AgentComplete"
}

func isAgentConnectEvent(amiEvent goami.Response) bool {
	return amiEvent.Get("Event") == "AgentConnect"
}

func isAgentDumpEvent(amiEvent goami.Response) bool {
	return amiEvent.Get("Event") == "AgentDump"
}

func isAgentRingNoAnswerEvent(amiEvent goami.Response) bool {
	return amiEvent.Get("Event") == "AgentRingNoAnswer"
}

func isDTMFBeginEvent(amiEvent goami.Response) bool {
	return amiEvent.Get("Event") == "DTMFBegin"
}

func isDTMFEndEvent(amiEvent goami.Response) bool {
	return amiEvent.Get("Event") == "DTMFEnd"
}

func isNewChannelEvent(amiEvent goami.Response) bool {
	eventType := amiEvent.Get("Event")
	if eventType == "Newchannel" {
		channelState := amiEvent.Get("ChannelStateDesc")
		if channelState == "Ring" || channelState == "Ringing" {
			return true
		}
	}
	return false
}

func buildEvent(eventType string, sourceID string, amiEvent goami.Response) *Event {
	data := make(map[string]string)

	allKeys := []string{
		"Event", "Channel", "ChannelState", "ChannelStateDesc",
		"CallerIDNum", "CallerIDName", "ConnectedLineNum", "ConnectedLineName",
		"Uniqueid", "Exten", "Context", "Priority",
		"Cause", "Cause-txt", "Duration",
		"DialStatus", "DestChannel", "DestChannelState", "DestChannelStateDesc",
		"DestCallerIDNum", "DestCallerIDName", "DestUniqueid",
		"BridgeState", "BridgeType", "Channel1", "Channel2",
		"Queue", "Position", "Count", "Linkedid", "Language", "AccountCode", "Strategy",
		"Interface", "MemberName", "HoldTime",
		"Wait", "OriginalPosition", "RingTime", "TalkTime", "Reason",
		"StateInterface", "Status", "Paused", "PausedReason", "Penalty",
		"CallsTaken", "LastCall", "LastPause", "LoginTime", "InCall",
		"Ringinuse", "Membership", "Wrapuptime",
		"Max", "Calls", "Holdtime", "Completed", "Abandoned",
		"ServiceLevelPerf", "ServiceLevelPerf2",
		"Digit", "Direction", "DurationMSec", "Begin", "End",
	}

	for _, key := range allKeys {
		value := amiEvent.Get(key)
		if value != "" {
			data[key] = value
		}
	}

	event := &Event{
		Type:      eventType,
		Source:    sourceID,
		Timestamp: time.Now(),
		Data:      data,
	}

	// log.Printf("[%s] Evento %s processado: %s",
	// 	sourceID, eventType, data)

	return event
}

package ami

import (
	"log"
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

	log.Printf("[%s] Evento %s processado: Channel=%s, CallerID=%s",
		sourceID, eventType, data["Channel"], data["CallerIDNum"])

	return event
}

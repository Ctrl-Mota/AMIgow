package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/safehouse/amigow/internal/ami"
)

func TestEnrichAgentCalledEventsPreservesOrderAndAddsCallID(t *testing.T) {
	src := make(chan ami.Event, 2)
	dst := make(chan ami.Event, 2)
	src <- ami.Event{
		Type: "agent_called",
		Data: map[string]string{"DestChannel": "PJSIP/101-00000001"},
	}
	src <- ami.Event{Type: "hangup", Data: map[string]string{"Uniqueid": "second"}}
	close(src)

	getter := func(context.Context, *ami.AsteriskManager, string, string) (ami.Response, error) {
		return ami.Response{"Value": []string{"call-123@example.invalid"}}, nil
	}
	enrichAgentCalledEvents(context.Background(), nil, src, dst, time.Second, getter)

	first := <-dst
	second := <-dst
	if first.Type != "agent_called" || first.Data["SipCallID"] != "call-123@example.invalid" {
		t.Fatalf("primeiro evento não foi enriquecido: %#v", first)
	}
	if second.Type != "hangup" || second.Data["Uniqueid"] != "second" {
		t.Fatalf("ordem dos eventos foi alterada: %#v", second)
	}
}

func TestEnrichAgentCalledEventsForwardsOnTimeout(t *testing.T) {
	src := make(chan ami.Event, 1)
	dst := make(chan ami.Event, 1)
	src <- ami.Event{
		Type: "agent_called",
		Data: map[string]string{"DestChannel": "PJSIP/101-00000001"},
	}
	close(src)

	getter := func(ctx context.Context, _ *ami.AsteriskManager, _, _ string) (ami.Response, error) {
		<-ctx.Done()
		return nil, errors.New("timeout")
	}
	enrichAgentCalledEvents(context.Background(), nil, src, dst, 10*time.Millisecond, getter)

	event := <-dst
	if event.Type != "agent_called" {
		t.Fatalf("evento original não foi encaminhado: %#v", event)
	}
	if _, exists := event.Data["SipCallID"]; exists {
		t.Fatalf("evento com falha não deveria conter Call-ID: %#v", event)
	}
}

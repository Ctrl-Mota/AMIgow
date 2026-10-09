package webhook

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/safehouse/amigow/internal/ami"
)

func TestBuildWebhookPayloadIncludesOptionalSipCallID(t *testing.T) {
	payload := buildWebhookPayload(ami.Event{
		Type: "agent_called",
		Data: map[string]string{
			"DestChannel": "PJSIP/101-00000001",
			"SipCallID":   "call-123@example.invalid",
		},
	})

	if payload.SipCallID != "call-123@example.invalid" {
		t.Fatalf("Call-ID não foi copiado para o payload: %#v", payload)
	}
}

func TestWebhookPayloadRemainsCompatibleWithoutSipCallID(t *testing.T) {
	payload := buildWebhookPayload(ami.Event{
		Type: "agent_called",
		Data: map[string]string{"DestChannel": "PJSIP/101-00000001"},
	})

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("falha ao serializar payload: %v", err)
	}
	if strings.Contains(string(raw), "sip_call_id") {
		t.Fatalf("campo opcional vazio não deveria alterar o payload: %s", raw)
	}
}

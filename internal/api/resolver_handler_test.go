package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/safehouse/amigow/internal/config"
)

func withAPIConfig(t *testing.T, cfg *config.Config) {
	t.Helper()
	previous := config.Current
	config.Current = cfg
	t.Cleanup(func() { config.Current = previous })
}

func TestDynamicResolverPreservaCamposDesconhecidos(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != "test-key" {
			t.Fatalf("X-API-Key ausente")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"config":{"id":12,"dial_timeout":45,"campo_futuro":"mantido"},"contacts":[{"id":91,"type":"cellphone","dial":"031999999999","modoLigacao":0},{"id":92,"type":"cellphone","dial":"031988888888","modoLigacao":1,"moradorId":"7"},{"id":93,"type":"cellphone","dial":"031977777777","modoLigacao":2,"moradorId":"8"}]}`))
	}))
	defer server.Close()

	withAPIConfig(t, &config.Config{
		APIKey: "test-key",
		ApiConnect: config.ApiConnect{
			Host:         server.URL,
			PathResolver: "/resolver",
		},
	})

	response, err := (&Handler{}).HandleDynamicResolver(context.Background(), &DynamicResolverInput{
		QuickNumber: "1001",
		Caller:      "2000",
		Linkedid:    "call.1",
	})
	if err != nil {
		t.Fatalf("HandleDynamicResolver retornou erro: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(response.Body, &payload); err != nil {
		t.Fatalf("resposta inválida: %v", err)
	}
	configPayload := payload["config"].(map[string]any)
	if configPayload["campo_futuro"] != "mantido" {
		t.Fatalf("campo desconhecido foi perdido: %#v", configPayload)
	}
	if configPayload["dial_timeout"] != float64(45) {
		t.Fatalf("dial_timeout foi alterado: %#v", configPayload)
	}
	if _, exists := configPayload["modo_video"]; exists {
		t.Fatalf("modo_video não pertence mais ao contrato do resolver: %#v", configPayload)
	}
	contacts := payload["contacts"].([]any)
	if len(contacts) != 3 {
		t.Fatalf("quantidade de contatos alterada: %#v", contacts)
	}
	for index, expectedMode := range []float64{0, 1, 2} {
		contact := contacts[index].(map[string]any)
		if contact["modoLigacao"] != expectedMode || contact["dial"] == "" {
			t.Fatalf("contrato de modo/prefixo foi alterado: %#v", contact)
		}
	}
}

func TestPortariaWakeupEncaminhaContratoSemConfiguracaoSipEComChave(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("método = %s", r.Method)
		}
		if r.Header.Get("X-API-Key") != "test-key" {
			t.Fatalf("X-API-Key ausente")
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("body inválido: %v", err)
		}
		if body["moradorId"] != float64(7) || body["configId"] != float64(12) ||
			body["ramal"] != "9001" || body["sipPassword"] != "dummy-secret" {
			t.Fatalf("body inesperado: %#v", body)
		}
		if _, ok := body["sipWss"]; ok {
			t.Fatalf("sipWss não deve ser enviado pelo AMIgow: %#v", body)
		}
		if _, ok := body["sipDomain"]; ok {
			t.Fatalf("sipDomain não deve ser enviado pelo AMIgow: %#v", body)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"sessionId":"session-1","campoNovo":true}`))
	}))
	defer server.Close()

	withAPIConfig(t, &config.Config{
		APIKey: "test-key",
		ApiConnect: config.ApiConnect{
			Host:       server.URL,
			PathWakeup: "/wakeup",
		},
	})

	response, err := (&Handler{}).HandlePortariaWakeup(context.Background(), &PortariaWakeupInput{
		MoradorID:   "7",
		ConfigID:    "12",
		Linkedid:    "call.1",
		Caller:      "2000",
		Ramal:       "9001",
		SipPassword: "dummy-secret",
	})
	if err != nil {
		t.Fatalf("HandlePortariaWakeup retornou erro: %v", err)
	}
	if string(response.Body) != `{"sessionId":"session-1","campoNovo":true}` {
		t.Fatalf("resposta não atravessou intacta: %s", response.Body)
	}
}

func TestPortariaWakeupFinalizeEncaminhaContratoV3(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("método = %s", r.Method)
		}
		if r.URL.Path != "/api/amigow/wakeup/finalize" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if r.Header.Get("X-API-Key") != "test-key" {
			t.Fatalf("X-API-Key ausente")
		}

		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("body inválido: %v", err)
		}
		if body["sessionId"] != "session-1" || body["linkedid"] != "call.1" ||
			body["reason"] != "origin_hangup" {
			t.Fatalf("body inesperado: %#v", body)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer server.Close()

	withAPIConfig(t, &config.Config{
		APIKey: "test-key",
		ApiConnect: config.ApiConnect{
			Host:               server.URL,
			PathWakeupFinalize: "/api/amigow/wakeup/finalize",
		},
	})

	response, err := (&Handler{}).HandlePortariaWakeupFinalize(
		context.Background(),
		&PortariaWakeupFinalizeInput{
			SessionID: "session-1",
			Linkedid:  "call.1",
			Reason:    "origin_hangup",
		},
	)
	if err != nil {
		t.Fatalf("HandlePortariaWakeupFinalize retornou erro: %v", err)
	}
	if string(response.Body) != `{"success":true}` {
		t.Fatalf("resposta não atravessou intacta: %s", response.Body)
	}
}

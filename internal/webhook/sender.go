package webhook

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/safehouse/amigow/internal/ami"
	"github.com/safehouse/amigow/internal/config"
)

type WebhookPayload struct {
	EventType   string            `json:"event_type"`
	Source      string            `json:"source"`
	Timestamp   string            `json:"timestamp"`
	Channel     string            `json:"channel,omitempty"`
	CallerID    string            `json:"caller_id,omitempty"`
	CallerName  string            `json:"caller_name,omitempty"`
	Destination string            `json:"destination,omitempty"`
	Cause       string            `json:"cause,omitempty"`
	CauseText   string            `json:"cause_text,omitempty"`
	Duration    string            `json:"duration,omitempty"`
	RawData     map[string]string `json:"raw_data"`
}

func ProcessEvents(eventChan <-chan ami.Event, cfg *config.Config) {
	log.Println("[WEBHOOK] Iniciando processamento de eventos")

	webhookMap := buildWebhookMap(cfg)

	for event := range eventChan {
		webhooks, found := webhookMap[event.Source]
		if !found {
			log.Printf("[WEBHOOK] Nenhum webhook configurado para %s", event.Source)
			continue
		}

		for _, webhook := range webhooks {
			if shouldSendEvent(event.Type, webhook.EventsFilter) {
				go sendWebhook(event, webhook)
			}
		}
	}
}

func buildWebhookMap(cfg *config.Config) map[string][]config.Webhook {
	webhookMap := make(map[string][]config.Webhook)

	for _, server := range cfg.AMIServers {
		webhookMap[server.ID] = server.Webhooks
	}

	return webhookMap
}

func shouldSendEvent(eventType string, filter []string) bool {
	if len(filter) == 0 {
		return false
	}

	for _, f := range filter {
		if f == "*" {
			return true
		}
		if f == eventType {
			return true
		}
	}

	return false
}

func sendWebhook(event ami.Event, webhook config.Webhook) {
	payload := buildWebhookPayload(event)

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[WEBHOOK] Erro ao serializar payload: %v", err)
		return
	}

	timeout := time.Duration(webhook.TimeoutSeconds) * time.Second
	client := &http.Client{
		Timeout: timeout,
	}

	success := attemptSend(client, webhook.URL, jsonPayload, event.Source, event.Type)

	if !success {
		// log.Printf("[WEBHOOK] Tentando retry para %s", webhook.URL)
		// time.Sleep(1 * time.Second)
		// attemptSend(client, webhook.URL, jsonPayload, event.Source, event.Type)
	}
}

func buildWebhookPayload(event ami.Event) WebhookPayload {
	return WebhookPayload{
		EventType:   event.Type,
		Source:      event.Source,
		Timestamp:   event.Timestamp.Format(time.RFC3339),
		Channel:     event.Data["Channel"],
		CallerID:    event.Data["CallerIDNum"],
		CallerName:  event.Data["CallerIDName"],
		Destination: event.Data["Exten"],
		Cause:       event.Data["Cause"],
		CauseText:   event.Data["Cause-txt"],
		Duration:    event.Data["Duration"],
		RawData:     event.Data,
	}
}

func attemptSend(client *http.Client, url string, payload []byte, source string, eventType string) bool {
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		log.Printf("[%s] Erro ao criar request: %v", source, err)
		return false
	}
	//log.Printf("[%s] Enviando webhook: \n\n%s", source, string(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Event-Type", eventType)
	req.Header.Set("X-Source", source)

	resp, err := client.Do(req)
	if err != nil {
		//log.Printf("[%s] Webhook %s falhou para %s: %v", source, eventType, url, err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("[%s] Webhook %s retornou status %d para %s", source, eventType, resp.StatusCode, url)
		return false
	}

	log.Printf("[%s] Webhook %s enviado com sucesso para %s", source, eventType, url)
	return true
}

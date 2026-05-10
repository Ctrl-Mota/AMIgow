package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

func (h *Handler) HandleDynamicResolver(ctx context.Context, input *DynamicResolverInput) (*ResolverResponse, error) {
	if input.Linkedid == "" {
		return nil, errorBadRequest("parâmetro linkedid é obrigatório")
	}

	cfg := h.Config.ApiConnect
	url := cfg.Host + cfg.PathResolver

	query := map[string]string{
		"quicknumber": input.QuickNumber,
		"caller":      input.Caller,
		"linkedid":    input.Linkedid,
	}

	log.Printf("[RESOLVER] GET %s linkedid=%s quicknumber=%s", url, input.Linkedid, input.QuickNumber)

	raw, err := getJSON(ctx, url, query, cfg.TimeoutSeconds, h.Config.ID)
	if err != nil {
		log.Printf("[RESOLVER] Erro ao chamar API externa: %v", err)
		return nil, errorInternal(fmt.Sprintf("erro ao resolver: %v", err))
	}
	log.Printf("[RESOLVER] Resposta: %s", string(raw))
	result := &ResolverResponse{}
	if err := json.Unmarshal(raw, &result.Body); err != nil {
		log.Printf("[RESOLVER] Resposta não compatível com o schema esperado: %v", err)
		return nil, errorInternal("resposta da API de resolução em formato inválido")
	}
	return result, nil
}

func (h *Handler) HandleOpenGate(ctx context.Context, input *OpenGateInput) (*ResolverResponse, error) {
	if input.Linkedid == "" {
		return nil, errorBadRequest("parâmetro linkedid é obrigatório")
	}

	cfg := h.Config.ApiConnect
	url := cfg.Host + cfg.PathOpenGate

	body := map[string]string{
		"device_id": input.DeviceID,
		"linkedid":  input.Linkedid,
	}

	log.Printf("[OPEN-GATE] Chamando %s com linkedid=%s device_id=%s", url, input.Linkedid, input.DeviceID)

	raw, err := postJSON(ctx, url, body, cfg.TimeoutSeconds, h.Config.ID)
	if err != nil {
		log.Printf("[OPEN-GATE] Erro ao chamar API externa: %v", err)
		return nil, errorInternal(fmt.Sprintf("erro ao abrir cancela: %v", err))
	}

	result := &ResolverResponse{}
	if err := json.Unmarshal(raw, &result.Body); err != nil {
		log.Printf("[OPEN-GATE] Resposta não compatível com o schema esperado: %v", err)
		return nil, errorInternal("resposta da API de cancela em formato inválido")
	}
	return result, nil
}

func outboundHTTPTimeout(fullURL string, timeoutSeconds int) time.Duration {
	d := 30 * time.Second
	if timeoutSeconds > 0 {
		d = time.Duration(timeoutSeconds) * time.Second
	}
	if strings.Contains(strings.ToLower(fullURL), "ngrok") && d < 45*time.Second {
		d = 45 * time.Second
	}
	return d
}

func applyOutboundHeaders(req *http.Request, sourceID string) {
	req.Header.Set("X-Source", sourceID)
	if strings.Contains(strings.ToLower(req.URL.Host), "ngrok") {
		req.Header.Set("ngrok-skip-browser-warning", "true")
	}
}

func postJSON(ctx context.Context, url string, body map[string]string, timeoutSeconds int, sourceID string) ([]byte, error) {
	timeout := outboundHTTPTimeout(url, timeoutSeconds)
	client := &http.Client{Timeout: timeout}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("erro ao serializar body: %w", err)
	}

	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("erro ao criar request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	applyOutboundHeaders(req, sourceID)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("erro na requisição: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler resposta: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API externa retornou status %d: %s", resp.StatusCode, string(respBody))
	}

	if !json.Valid(respBody) {
		return nil, fmt.Errorf("resposta não é JSON válido")
	}

	return respBody, nil
}

func getJSON(ctx context.Context, url string, queryParams map[string]string, timeoutSeconds int, sourceID string) ([]byte, error) {
	timeout := outboundHTTPTimeout(url, timeoutSeconds)
	client := &http.Client{Timeout: timeout}

	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("erro ao criar request: %w", err)
	}
	q := req.URL.Query()
	for key, value := range queryParams {
		q.Set(key, value)
	}
	req.URL.RawQuery = q.Encode()
	applyOutboundHeaders(req, sourceID)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("erro na requisição: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler resposta: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API externa retornou status %d: %s", resp.StatusCode, string(respBody))
	}

	if !json.Valid(respBody) {
		return nil, fmt.Errorf("resposta não é JSON válido")
	}

	return respBody, nil
}

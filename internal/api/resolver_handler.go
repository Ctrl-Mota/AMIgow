package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/safehouse/amigow/internal/config"
)

var condominiosSlugsCache struct {
	sync.Mutex
	data      []byte
	expiresAt time.Time
}

func (h *Handler) HandleDynamicResolver(ctx context.Context, input *DynamicResolverInput) (*RawJSONResponse, error) {
	if input.Linkedid == "" {
		return nil, errorBadRequest("parâmetro linkedid é obrigatório")
	}

	cfg := config.Current.ApiConnect
	url := cfg.Host + cfg.PathResolver

	query := map[string]string{
		"quicknumber": input.QuickNumber,
		"caller":      input.Caller,
		"linkedid":    input.Linkedid,
	}

	log.Printf("[RESOLVER] GET %s linkedid=%s quicknumber=%s", url, input.Linkedid, input.QuickNumber)

	raw, err := getJSON(ctx, url, query, cfg.TimeoutSeconds)
	if err != nil {
		log.Printf("[RESOLVER] Erro ao chamar API externa: %v", err)
		return nil, errorInternal(fmt.Sprintf("erro ao resolver: %v", err))
	}
	// Não desserializar para um schema local: o dialplan é o consumidor do
	// contrato e campos novos da API precisam atravessar o proxy intactos.
	return &RawJSONResponse{Body: json.RawMessage(raw)}, nil
}

func (h *Handler) HandlePortariaWakeup(ctx context.Context, input *PortariaWakeupInput) (*RawJSONResponse, error) {
	if input.MoradorID == "" || input.ConfigID == "" || input.Linkedid == "" ||
		input.Caller == "" || input.Ramal == "" || input.SipPassword == "" {
		return nil, errorBadRequest("parâmetros obrigatórios ausentes no wake-up")
	}
	moradorID, err := strconv.Atoi(input.MoradorID)
	if err != nil || moradorID <= 0 {
		return nil, errorBadRequest("moradorId inválido")
	}
	configID, err := strconv.Atoi(input.ConfigID)
	if err != nil || configID <= 0 {
		return nil, errorBadRequest("configId inválido")
	}

	cfg := config.Current.ApiConnect
	url := cfg.Host + cfg.PathWakeup
	if cfg.PathWakeup == "" {
		return nil, errorInternal("path_wakeup não configurado")
	}

	// Nunca registrar senha SIP nem URL completa com query.
	log.Printf("[PORTARIA-WAKEUP] POST configId=%s linkedid=%s ramal=%s", input.ConfigID, input.Linkedid, input.Ramal)

	body := map[string]any{
		"moradorId":   moradorID,
		"configId":    configID,
		"linkedid":    input.Linkedid,
		"caller":      input.Caller,
		"ramal":       input.Ramal,
		"sipPassword": input.SipPassword,
	}

	raw, err := postJSON(ctx, url, body, cfg.TimeoutSeconds)
	if err != nil {
		log.Printf("[PORTARIA-WAKEUP] API externa recusou linkedid=%s: %v", input.Linkedid, err)
		return nil, errorInternal(fmt.Sprintf("erro ao iniciar wake-up: %v", err))
	}

	return &RawJSONResponse{Body: json.RawMessage(raw)}, nil
}

func (h *Handler) HandlePortariaWakeupFinalize(ctx context.Context, input *PortariaWakeupFinalizeInput) (*RawJSONResponse, error) {
	if input.SessionID == "" || input.Linkedid == "" {
		return nil, errorBadRequest("sessionId e linkedid são obrigatórios")
	}

	cfg := config.Current.ApiConnect
	url := cfg.Host + cfg.PathWakeupFinalize
	if cfg.PathWakeupFinalize == "" {
		return nil, errorInternal("path_wakeup_finalize não configurado")
	}

	log.Printf("[PORTARIA-WAKEUP] Encerrando sessionId=%s linkedid=%s reason=%s", input.SessionID, input.Linkedid, input.Reason)
	raw, err := postJSON(ctx, url, map[string]string{
		"sessionId": input.SessionID,
		"linkedid":  input.Linkedid,
		"reason":    input.Reason,
	}, cfg.TimeoutSeconds)
	if err != nil {
		return nil, errorInternal(fmt.Sprintf("erro ao finalizar wake-up: %v", err))
	}

	return &RawJSONResponse{Body: json.RawMessage(raw)}, nil
}

func (h *Handler) HandleOpenGate(ctx context.Context, input *OpenGateInput) (*ResolverResponse, error) {
	if input.Linkedid == "" {
		return nil, errorBadRequest("parâmetro linkedid é obrigatório")
	}

	cfg := config.Current.ApiConnect
	url := cfg.Host + cfg.PathOpenGate

	body := map[string]string{
		"device_id": input.DeviceID,
		"linkedid":  input.Linkedid,
	}

	log.Printf("[OPEN-GATE] Chamando %s com linkedid=%s device_id=%s", url, input.Linkedid, input.DeviceID)

	raw, err := postJSON(ctx, url, body, cfg.TimeoutSeconds)
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

func (h *Handler) HandleCondominiosSlugs(ctx context.Context, input *struct{}) (*CondominiosSlugsResponse, error) {
	condominiosSlugsCache.Lock()
	if condominiosSlugsCache.data != nil && time.Now().Before(condominiosSlugsCache.expiresAt) {
		cached := condominiosSlugsCache.data
		condominiosSlugsCache.Unlock()

		var slugs []string
		if err := json.Unmarshal(cached, &slugs); err != nil {
			return nil, errorInternal("erro ao ler cache de condominios slugs")
		}
		result := &CondominiosSlugsResponse{}
		result.Body = slugs
		return result, nil
	}
	condominiosSlugsCache.Unlock()

	cfg := config.Current.ApiConnect
	url := cfg.Host + cfg.PathCondominiosSlugs

	log.Printf("[CONDOMINIOS-SLUGS] GET %s", url)

	raw, err := getJSON(ctx, url, nil, cfg.TimeoutSeconds)
	if err != nil {
		log.Printf("[CONDOMINIOS-SLUGS] Erro ao chamar API externa: %v", err)
		return nil, errorInternal(fmt.Sprintf("erro ao buscar slugs: %v", err))
	}

	var slugs []string
	if err := json.Unmarshal(raw, &slugs); err != nil {
		log.Printf("[CONDOMINIOS-SLUGS] Resposta não compatível com []string: %v", err)
		return nil, errorInternal("resposta da API em formato inválido")
	}

	condominiosSlugsCache.Lock()
	condominiosSlugsCache.data = raw
	condominiosSlugsCache.expiresAt = time.Now().Add(2 * time.Minute)
	condominiosSlugsCache.Unlock()

	result := &CondominiosSlugsResponse{}
	result.Body = slugs
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

func applyOutboundHeaders(req *http.Request) {
	req.Header.Set("X-API-Key", config.Current.APIKey)
	if strings.Contains(strings.ToLower(req.URL.Host), "ngrok") {
		req.Header.Set("ngrok-skip-browser-warning", "true")
	}
}

func postJSON(ctx context.Context, url string, body any, timeoutSeconds int) ([]byte, error) {
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
	applyOutboundHeaders(req)

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

func getJSON(ctx context.Context, url string, queryParams map[string]string, timeoutSeconds int) ([]byte, error) {
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
	applyOutboundHeaders(req)

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

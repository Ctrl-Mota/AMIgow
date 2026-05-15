package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/safehouse/amigow/internal/ami"
	"github.com/safehouse/amigow/internal/api"
	"github.com/safehouse/amigow/internal/cdr"
	"github.com/safehouse/amigow/internal/config"
	"github.com/safehouse/amigow/internal/webhook"
)

func main() {
	log.SetOutput(os.Stdout)
	log.Println("=== AMIgow - Iniciando ===")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// configAPIURL := os.Getenv("CONFIG_API_URL")
	// if configAPIURL == "" {
	// 	configAPIURL = "http://localhost:9000/config"
	// }

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Erro ao carregar configuração: %v", err)
	}
	config.Current = cfg

	log.Printf("Configuração carregada: servidor AMI %s", config.Current.ID)

	eventChan := make(chan ami.Event, 1000)

	manager, err := ami.NewAsteriskManager(ctx, config.Current.ID, config.Current.AMIServer)
	if err != nil {
		log.Fatalf("Falha ao conectar AMI %s: %v", config.Current.ID, err)
	}
	manager.Start(eventChan)

	go webhook.ProcessEvents(eventChan)

	cdrDB, err := cdr.Open()
	if err != nil {
		log.Printf("AVISO: CDR database não disponível: %v", err)
		cdrDB = nil
	}

	handler := api.NewHandler(manager, cdrDB)

	router := chi.NewRouter()

	basePath := config.Current.BasePath
	if basePath == "" {
		basePath = ""
	}

	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path

			publicPaths := []string{
				"/docs", "/openapi", "/openapi.json", "/openapi.yaml", "/$openapi", "/health", "/webhooks/schema",
				// "/dynamic-resolver", "/open-gate", "/cdr/search",
			}

			for _, pubPath := range publicPaths {
				if strings.HasPrefix(path, pubPath) || path == pubPath {
					next.ServeHTTP(w, r)
					return
				}
			}

			if strings.HasPrefix(path, "/schemas") {
				next.ServeHTTP(w, r)
				return
			}

			handler.ValidateAPIKey(next).ServeHTTP(w, r)
		})
	})

	apiConfig := huma.DefaultConfig("AMIgow API", "1.0.0")

	apiConfig.Components.SecuritySchemes = map[string]*huma.SecurityScheme{
		"apiKey": {
			Type: "apiKey",
			In:   "header",
			Name: "X-API-Key",
		},
	}
	apiConfig.Info = &huma.Info{
		Title:       "AMIgow API",
		Version:     "1.0.0",
		Description: "API para se comunicar com as aplicações Asterisk via AMI (Webhooks e REST API)",
		Contact: &huma.Contact{
			Name:  "SafeHouse",
			URL:   "https://safehouseapp.com.br",
			Email: "contato@appsafehouse.com.br",
		},
	}

	if config.Current.ServerHost != "" {
		serverURL := config.Current.ServerHost
		if basePath != "" {
			serverURL = serverURL + basePath
		}
		apiConfig.Servers = []*huma.Server{
			{URL: serverURL, Description: "Servidor de produção"},
		}
	}

	humaAPI := humachi.New(router, apiConfig)

	if basePath != "" {
		humaAPI.OpenAPI().OpenAPI = "3.1.0"
		for _, path := range humaAPI.OpenAPI().Paths {
			for _, op := range []*huma.Operation{path.Get, path.Post, path.Put, path.Delete, path.Patch} {
				if op != nil {
					continue
				}
			}
		}
	}

	huma.Register(humaAPI, huma.Operation{
		OperationID: "post-action",
		Method:      http.MethodPost,
		Path:        "/action",
		Summary:     "Executa ação AMI",
		Tags:        []string{"AMI"},
		Security: []map[string][]string{
			{"apiKey": {}},
		},
	}, handler.HandleAction)

	huma.Register(humaAPI, huma.Operation{
		OperationID: "post-queue-add",
		Method:      http.MethodPost,
		Path:        "/queue/add",
		Summary:     "Queue · Adiciona interface à fila",
		Tags:        []string{"AMI"},
		Security: []map[string][]string{
			{"apiKey": {}},
		},
	}, handler.HandleQueueAdd)

	huma.Register(humaAPI, huma.Operation{
		OperationID: "post-queue-remove",
		Method:      http.MethodPost,
		Path:        "/queue/remove",
		Summary:     "Queue · Remove interface da fila",
		Tags:        []string{"AMI"},
		Security: []map[string][]string{
			{"apiKey": {}},
		},
	}, handler.HandleQueueRemove)

	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-queue-status",
		Method:      http.MethodPost,
		Path:        "/queue/status",
		Summary:     "Queue · Obtém status da fila",
		Tags:        []string{"AMI"},
		Security: []map[string][]string{
			{"apiKey": {}},
		},
	}, handler.HandleQueueStatus)

	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-health",
		Method:      http.MethodGet,
		Path:        "/health",
		Summary:     "Verifica saúde do serviço",
		Tags:        []string{"Healthcheck"},
	}, handler.HandleHealth)

	huma.Register(humaAPI, huma.Operation{
		OperationID: "post-webhook",
		Method:      http.MethodPost,
		Path:        "/amigow/webhook",
		Summary:     "Envia evento de webhook",
		Description: "Um evento que o AMIgow envia para o endpoint configurado",
		Tags:        []string{"Webhooks"},
		Security: []map[string][]string{
			{"apiKey": {}},
		},
	}, handler.HandleWebhookSchema)

	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-channel-redirect",
		Method:      http.MethodPost,
		Path:        "/channel/redirect",
		Summary:     "Channel · Redireciona (transfere) um canal",
		Description: "Executa o comando AMI Redirect para transferir um canal ativo para outro ramal/contexto",
		Tags:        []string{"AMI"},
		Security: []map[string][]string{
			{"apiKey": {}},
		},
	}, handler.HandleChannelRedirect)

	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-dynamic-resolver",
		Method:      http.MethodGet,
		Path:        "/dynamic-resolver",
		Summary:     "Resolve contatos de um ramal PBX",
		Description: "Chamado pelo dialplan do Asterisk para obter a lista de contatos de um quicknumber. Sem autenticação (uso interno).",
		Tags:        []string{"Asterisk Egress · Portaria Autônoma"},
		Security: []map[string][]string{
			{"apiKey": {}},
		},
	}, handler.HandleDynamicResolver)

	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-open-gate",
		Method:      http.MethodGet,
		Path:        "/open-gate",
		Summary:     "Abre cancela/portão",
		Description: "Chamado pelo dialplan do Asterisk para acionar a abertura de uma cancela. Sem autenticação (uso interno).",
		Tags:        []string{"Asterisk Egress · Portaria Autônoma"},
		Security: []map[string][]string{
			{"apiKey": {}},
		},
	}, handler.HandleOpenGate)

	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-basic-lists-condominios-slugs",
		Method:      http.MethodGet,
		Path:        "/basic-lists/condominios-slugs",
		Summary:     "Lista slugs de condomínios",
		Description: "Retorna a lista de slugs de condomínios. Resultado cacheado por 2 minutos.",
		Tags:        []string{"Basic Lists"},
		Security: []map[string][]string{
			{"apiKey": {}},
		},
	}, handler.HandleCondominiosSlugs)

	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-cdr-search",
		Method:      http.MethodGet,
		Path:        "/cdr/search",
		Summary:     "Busca CDR por linkedid",
		Description: "Retorna todos os registros de CDR do Asterisk para um dado linkedid",
		Tags:        []string{"Reports"},
		Security: []map[string][]string{
			{"apiKey": {}},
		},
	}, handler.HandleCDRSearch)

	serverAddr := ":8080"
	log.Printf("Servidor HTTP iniciado em %s", serverAddr)
	if basePath != "" {
		log.Printf("Base path configurado: %s", basePath)
		log.Printf("Documentação disponível em http://localhost%s%s/docs", serverAddr, basePath)
	} else {
		log.Printf("Documentação disponível em http://localhost%s/docs", serverAddr)
	}
	log.Printf("OpenAPI spec em http://localhost%s/openapi.json", serverAddr)

	go func() {
		err := http.ListenAndServe(serverAddr, router)
		if err != nil {
			log.Fatalf("Erro ao iniciar servidor HTTP: %v", err)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigChan
	log.Printf("Sinal recebido: %v", sig)
	log.Println("Encerrando conexões...")

	cancel()

	log.Printf("Fechando conexão com %s", config.Current.ID)
	if err := manager.Close(); err != nil {
		log.Printf("Erro ao fechar %s: %v", config.Current.ID, err)
	}

	close(eventChan)

	log.Println("=== AMIgow - Encerrado ===")
}

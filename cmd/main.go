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
	"github.com/safehouse/amigow/internal/config"
	"github.com/safehouse/amigow/internal/webhook"
)

func main() {
	log.SetOutput(os.Stdout)
	log.Println("=== AMIgow - Iniciando ===")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	configAPIURL := os.Getenv("CONFIG_API_URL")
	if configAPIURL == "" {
		configAPIURL = "http://localhost:9000/config"
	}

	cfg, err := config.Load(configAPIURL)
	if err != nil {
		log.Fatalf("Erro ao carregar configuração: %v", err)
	}

	log.Printf("Configuração carregada: %d servidores AMI", len(cfg.AMIServers))

	eventChan := make(chan ami.Event, 1000)

	managers := make(map[string]*ami.AsteriskManager)

	for _, server := range cfg.AMIServers {
		manager, err := ami.NewAsteriskManager(ctx, server)
		if err != nil {
			log.Printf("AVISO: Falha ao conectar em %s: %v", server.ID, err)
			continue
		}

		managers[server.ID] = manager
		manager.Start(eventChan)
	}

	if len(managers) == 0 {
		log.Fatal("Nenhuma conexão AMI estabelecida. Encerrando.")
	}

	go webhook.ProcessEvents(eventChan, cfg)

	handler := api.NewHandler(managers, cfg)

	router := chi.NewRouter()

	basePath := cfg.BasePath
	if basePath == "" {
		basePath = ""
	}

	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path

			publicPaths := []string{"/docs", "/openapi", "/openapi.json", "/openapi.yaml", "/$openapi", "/health"}

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

	if cfg.ServerHost != "" {
		serverURL := cfg.ServerHost
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
		Summary:     "Adiciona interface à fila",
		Tags:        []string{"Queue"},
		Security: []map[string][]string{
			{"apiKey": {}},
		},
	}, handler.HandleQueueAdd)

	huma.Register(humaAPI, huma.Operation{
		OperationID: "post-queue-remove",
		Method:      http.MethodPost,
		Path:        "/queue/remove",
		Summary:     "Remove interface da fila",
		Tags:        []string{"Queue"},
		Security: []map[string][]string{
			{"apiKey": {}},
		},
	}, handler.HandleQueueRemove)

	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-health",
		Method:      http.MethodGet,
		Path:        "/health",
		Summary:     "Verifica saúde do serviço",
		Tags:        []string{"Health"},
	}, handler.HandleHealth)

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

	for id, manager := range managers {
		log.Printf("Fechando conexão com %s", id)
		if err := manager.Close(); err != nil {
			log.Printf("Erro ao fechar %s: %v", id, err)
		}
	}

	close(eventChan)

	log.Println("=== AMIgow - Encerrado ===")
}

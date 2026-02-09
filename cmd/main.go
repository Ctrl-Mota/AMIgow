package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/safehouse/amigow/internal/ami"
	"github.com/safehouse/amigow/internal/api"
	"github.com/safehouse/amigow/internal/config"
	"github.com/safehouse/amigow/internal/webhook"
)

func main() {
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

	handler := api.NewHandler(managers)

	http.HandleFunc("/action", handler.HandleAction)
	http.HandleFunc("/health", handler.HandleHealth)

	serverAddr := ":8080"
	log.Printf("Servidor HTTP iniciado em %s", serverAddr)

	go func() {
		err := http.ListenAndServe(serverAddr, nil)
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

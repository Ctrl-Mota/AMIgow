package ami

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"

	goami "github.com/heltonmarx/goami/ami"
	"github.com/safehouse/amigow/internal/config"
)

type AsteriskManager struct {
	ID       string
	Host     string
	Port     int
	Username string
	Password string
	socket   *LoggedSocket
	uuid     string
	ctx      context.Context
	cancel   context.CancelFunc
	webhooks []config.Webhook
	logFile  *os.File
	wg       sync.WaitGroup
}

func NewAsteriskManager(parentCtx context.Context, server config.AMIServer) (*AsteriskManager, error) {
	ctx, cancel := context.WithCancel(parentCtx)

	address := fmt.Sprintf("%s:%d", server.Host, server.Port)
	log.Printf("[%s] Conectando ao AMI em %s", server.ID, address)

	logFileName := fmt.Sprintf("ami_stream_%s.log", server.ID)
	logFile, err := os.OpenFile(logFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("erro ao criar arquivo de log: %w", err)
	}

	socket, err := NewLoggedSocket(ctx, address, logFile, server.ID)
	if err != nil {
		cancel()
		logFile.Close()
		return nil, fmt.Errorf("erro ao criar socket: %w", err)
	}

	connected, err := goami.Connect(ctx, socket)
	if err != nil || !connected {
		cancel()
		logFile.Close()
		return nil, fmt.Errorf("erro ao conectar: %w", err)
	}

	uuid, err := goami.GetUUID()
	if err != nil {
		cancel()
		logFile.Close()
		return nil, fmt.Errorf("erro ao gerar UUID: %w", err)
	}

	mgr := &AsteriskManager{
		ID:       server.ID,
		Host:     server.Host,
		Port:     server.Port,
		Username: server.Username,
		Password: server.Password,
		socket:   socket,
		uuid:     uuid,
		ctx:      ctx,
		cancel:   cancel,
		webhooks: server.Webhooks,
		logFile:  logFile,
	}

	log.Printf("[%s] Log de saatream será salvo em: %s", server.ID, logFileName)

	err = goami.Login(ctx, socket, server.Username, server.Password, "on", uuid)
	// err = login(ctx, socket, server.Username, server.Password, "on", uuid)
	if err != nil {
		cancel()
		logFile.Close()
		return nil, fmt.Errorf("erro no login: %w", err)
	}
	log.Printf("[%s] Logged in successfully", server.ID)

	return mgr, nil
}

// func login(ctx context.Context, client *LoggedSocket, user, secret, events, actionID string) error {

// 	err := client.Send("Action: Login\r\nActionID: " + actionID + "\r\nUsername: " + user + "\r\nAuthType: plain\r\nSecret: " + secret + "\r\nEvents: " + events + "\r\n\r\n")
// 	if err != nil {
// 		return err
// 	}

//		return nil
//	}
func (m *AsteriskManager) Start(eventChan chan<- Event) {
	m.wg.Add(1)
	go m.eventLoop(eventChan)
}

func (m *AsteriskManager) eventLoop(eventChan chan<- Event) {
	defer m.wg.Done()
	log.Printf("[%s] Iniciando loop de eventos", m.ID)

	for {
		select {
		case <-m.ctx.Done():
			log.Printf("[%s] Contexto cancelado, encerrando loop de eventos", m.ID)
			return
		default:
			amiEvent, err := goami.Events(m.ctx, m.socket)
			if err != nil {
				//log.Printf("[%s] Erro ao ler evento: %v", m.ID, err)

				// log.Printf("[%s] Tentando reconectar...", m.ID)
				// time.Sleep(2 * time.Second)

				// if reconnectErr := m.reconnect(); reconnectErr != nil {
				// 	log.Printf("[%s] Falha na reconexão: %v", m.ID, reconnectErr)
				// 	return
				// }
				continue
			}

			event, shouldProcess := ProcessAMIEvent(amiEvent, m.ID)
			if shouldProcess {
				eventChan <- *event
			}
		}
	}
}

func (m *AsteriskManager) reconnect() error {
	address := fmt.Sprintf("%s:%d", m.Host, m.Port)

	newSocket, err := NewLoggedSocket(m.ctx, address, m.logFile, m.ID)
	if err != nil {
		return fmt.Errorf("erro ao criar novo socket: %w", err)
	}

	connected, err := goami.Connect(m.ctx, newSocket)
	if err != nil || !connected {
		return fmt.Errorf("erro ao reconectar: %w", err)
	}

	newUUID, err := goami.GetUUID()
	if err != nil {
		return fmt.Errorf("erro ao gerar novo UUID: %w", err)
	}

	err = goami.Login(m.ctx, newSocket, m.Username, m.Password, "all", newUUID)
	if err != nil {
		return fmt.Errorf("erro no login após reconexão: %w", err)
	}

	m.socket = newSocket
	m.uuid = newUUID

	log.Printf("[%s] Reconectado com sucesso", m.ID)
	return nil
}

func (m *AsteriskManager) SendAction(action map[string]string) (map[string]string, error) {
	actionType := action["Action"]
	log.Printf("[%s] Enviando action: %s", m.ID, actionType)
	return SendActionRaw(m.ctx, m, action)
}

func (m *AsteriskManager) Close() error {
	log.Printf("[%s] Fechando conexão AMI", m.ID)

	m.cancel()
	m.wg.Wait()

	if err := goami.Logoff(m.ctx, m.socket, m.uuid); err != nil {
		log.Printf("[%s] Erro no logoff: %v", m.ID, err)
	}

	if m.logFile != nil {
		m.logFile.Close()
		log.Printf("[%s] Arquivo de log fechado", m.ID)
	}

	return m.socket.Close(m.ctx)
}

func (m *AsteriskManager) GetWebhooks() []config.Webhook {
	return m.webhooks
}

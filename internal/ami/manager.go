package ami

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/safehouse/amigow/internal/config"
)

const (
	loginEvents          = "on"
	defaultActionTimeout = 20 * time.Second
	logoffTimeout        = 3 * time.Second
	pingInterval         = 30 * time.Second
	pingTimeout          = 15 * time.Second
	reconnectDelay       = 2 * time.Second
	dispatcherTick       = time.Second
	frameChanSize        = 256
	actionChanSize       = 64
	maxListFrames        = 20000
	maxPreLoginFrames    = 64
)

var (
	ErrManagerClosed = errors.New("ami: manager encerrado")
	ErrNoConnection  = errors.New("ami: sem conexão com o AMI")
)

type actionRequest struct {
	action   string
	fields   []Field
	complete string
	internal bool
	ctx      context.Context
	deadline time.Time
	reply    chan actionResult
}

type actionResult struct {
	frames []Response
	err    error
}

func (r *actionRequest) finish(frames []Response, err error) {
	if r.reply == nil {
		return
	}
	r.reply <- actionResult{frames: frames, err: err}
}

type pendingAction struct {
	id       string
	action   string
	complete string
	frames   []Response
	ctx      context.Context
	deadline time.Time
	reply    chan actionResult
}

// accept devolve true quando a action já tem tudo que precisava.
func (p *pendingAction) accept(frame Response) (bool, error) {
	if p.complete == "" {
		p.frames = append(p.frames, frame)
		return true, nil
	}

	// Actions de lista respondem primeiro com um "Success" avisando que os
	// eventos vêm a seguir.
	if response := frame.Get("Response"); response != "" {
		if !strings.EqualFold(response, "Success") {
			return true, fmt.Errorf("ami: action %s recusada: %s", p.action, frame.Get("Message"))
		}
		return false, nil
	}

	if frame.Get("Event") == p.complete {
		return true, nil
	}

	p.frames = append(p.frames, frame)
	if len(p.frames) > maxListFrames {
		return true, fmt.Errorf("ami: action %s passou de %d eventos sem %s", p.action, maxListFrames, p.complete)
	}
	return false, nil
}

func (p *pendingAction) finish(frames []Response, err error) {
	if p.reply == nil {
		return
	}
	p.reply <- actionResult{frames: frames, err: err}
}

// session é uma conexão AMI viva com o seu reader. Trocar de conexão é trocar
// de session, então frames antigos nunca se misturam com os novos.
type session struct {
	conn      *LoggedSocket
	reader    *frameReader
	frames    chan Response
	errs      chan error
	done      chan struct{}
	closeOnce sync.Once
}

func newSession(conn *LoggedSocket, reader *frameReader) *session {
	return &session{
		conn:   conn,
		reader: reader,
		frames: make(chan Response, frameChanSize),
		errs:   make(chan error, 1),
		done:   make(chan struct{}),
	}
}

func (s *session) shutdown() {
	s.closeOnce.Do(func() {
		close(s.done)
		s.conn.Close()
	})
}

type AsteriskManager struct {
	ID       string
	Host     string
	Port     int
	Username string
	Password string

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	webhooks []config.Webhook

	actions chan *actionRequest

	bootSession *session
	bootFrames  []Response

	bootedAt  int64
	actionSeq uint64
	orphans   uint64

	closeOnce sync.Once
}

func NewAsteriskManager(parentCtx context.Context, id string, server config.AMIServer) (*AsteriskManager, error) {
	ctx, cancel := context.WithCancel(parentCtx)

	m := &AsteriskManager{
		ID:       id,
		Host:     server.Host,
		Port:     server.Port,
		Username: server.Username,
		Password: server.Password,
		ctx:      ctx,
		cancel:   cancel,
		webhooks: server.Webhooks,
		actions:  make(chan *actionRequest, actionChanSize),
		bootedAt: time.Now().Unix(),
	}

	current, early, err := m.connect()
	if err != nil {
		cancel()
		return nil, err
	}

	m.bootSession = current
	m.bootFrames = early

	return m, nil
}

func (m *AsteriskManager) connect() (*session, []Response, error) {
	address := fmt.Sprintf("%s:%d", m.Host, m.Port)
	log.Printf("[%s] Conectando ao AMI em %s", m.ID, address)

	conn, err := dialLoggedSocket(m.ctx, address, m.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("erro ao conectar em %s: %w", address, err)
	}

	reader := newFrameReader(conn)

	greeting, err := reader.ReadLine()
	if err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("erro ao ler saudação do AMI: %w", err)
	}
	if !strings.Contains(greeting, "Asterisk Call Manager") {
		conn.Close()
		return nil, nil, fmt.Errorf("saudação inesperada do AMI: %q", greeting)
	}

	early, err := m.login(conn, reader)
	if err != nil {
		conn.Close()
		return nil, nil, err
	}

	conn.SetReadTimeout(steadyReadTimeout)
	log.Printf("[%s] Login no AMI concluído (%s)", m.ID, strings.TrimSpace(greeting))

	return newSession(conn, reader), early, nil
}

func (m *AsteriskManager) login(conn *LoggedSocket, reader *frameReader) ([]Response, error) {
	id := m.nextActionID()

	message, err := encodeAction([]Field{
		{Key: "Action", Value: "Login"},
		{Key: "ActionID", Value: id},
		{Key: "Username", Value: m.Username},
		{Key: "Secret", Value: m.Password},
		{Key: "Events", Value: loginEvents},
	})
	if err != nil {
		return nil, err
	}
	if err := conn.Send(message); err != nil {
		return nil, fmt.Errorf("erro ao enviar login: %w", err)
	}

	var early []Response
	for {
		frame, err := reader.ReadFrame()
		if err != nil {
			return nil, fmt.Errorf("erro ao ler resposta do login: %w", err)
		}

		if frame.Get("ActionID") != id {
			// O Asterisk não deveria mandar nada antes da resposta do login,
			// mas se mandar o frame é guardado em vez de descartado.
			early = append(early, frame)
			if len(early) > maxPreLoginFrames {
				return nil, errors.New("ami: login sem resposta do Asterisk")
			}
			continue
		}

		if !strings.EqualFold(frame.Get("Response"), "Success") {
			return nil, fmt.Errorf("login recusado: %s", frame.Get("Message"))
		}
		return early, nil
	}
}

func (m *AsteriskManager) Start(eventChan chan<- Event) {
	m.wg.Add(1)
	go m.dispatch(eventChan)
}

// dispatch é a única goroutine que escreve no socket, guarda as actions
// pendentes e reconecta. A leitura fica com readLoop, que só entrega frames.
func (m *AsteriskManager) dispatch(eventChan chan<- Event) {
	defer m.wg.Done()
	log.Printf("[%s] Iniciando dispatcher AMI", m.ID)

	pending := make(map[string]*pendingAction)
	var current *session

	defer func() {
		if current != nil {
			current.shutdown()
		}
		failPending(pending, ErrManagerClosed)
		log.Printf("[%s] Dispatcher AMI encerrado", m.ID)
	}()

	nextConnectAt := time.Now()
	nextPingAt := time.Now().Add(pingInterval)

	teardown := func(reason error) {
		if current == nil {
			return
		}
		current.shutdown()
		current = nil
		failPending(pending, reason)
		nextConnectAt = time.Now().Add(reconnectDelay)
	}

	if m.bootSession != nil {
		current = m.bootSession
		m.bootSession = nil
		m.startReader(current)
		for _, frame := range m.bootFrames {
			if !m.deliverEvent(frame, eventChan) {
				return
			}
		}
		m.bootFrames = nil
	}

	ticker := time.NewTicker(dispatcherTick)
	defer ticker.Stop()

	for {
		var frames chan Response
		var errs chan error
		if current != nil {
			frames = current.frames
			errs = current.errs
		}

		select {
		case <-m.ctx.Done():
			return

		case frame := <-frames:
			if !m.route(frame, pending, eventChan) {
				return
			}

		case err := <-errs:
			log.Printf("[%s] Erro de leitura no AMI: %v", m.ID, err)
			teardown(fmt.Errorf("ami: conexão perdida: %w", err))

		case request := <-m.actions:
			if current == nil {
				request.finish(nil, ErrNoConnection)
				continue
			}
			if err := m.send(current, request, pending); err != nil {
				log.Printf("[%s] Erro ao enviar action %s: %v", m.ID, request.action, err)
				request.finish(nil, err)
				teardown(fmt.Errorf("ami: conexão perdida: %w", err))
			}

		case now := <-ticker.C:
			m.expirePending(pending, now)

			if current == nil {
				if now.Before(nextConnectAt) {
					continue
				}
				newSession, early, err := m.connect()
				if err != nil {
					log.Printf("[%s] Falha na reconexão: %v", m.ID, err)
					nextConnectAt = time.Now().Add(reconnectDelay)
					continue
				}
				current = newSession
				m.startReader(current)
				nextPingAt = time.Now().Add(pingInterval)
				log.Printf("[%s] Reconectado ao AMI", m.ID)
				for _, frame := range early {
					if !m.deliverEvent(frame, eventChan) {
						return
					}
				}
				continue
			}

			if now.After(nextPingAt) {
				nextPingAt = now.Add(pingInterval)
				heartbeat := &actionRequest{
					action:   "Ping",
					internal: true,
					deadline: now.Add(pingTimeout),
				}
				if err := m.send(current, heartbeat, pending); err != nil {
					log.Printf("[%s] Erro no heartbeat: %v", m.ID, err)
					teardown(fmt.Errorf("ami: conexão perdida: %w", err))
				}
			}
		}
	}
}

func (m *AsteriskManager) startReader(s *session) {
	m.wg.Add(1)
	go m.readLoop(s)
}

func (m *AsteriskManager) readLoop(s *session) {
	defer m.wg.Done()

	for {
		frame, err := s.reader.ReadFrame()
		if err != nil {
			select {
			case s.errs <- err:
			case <-s.done:
			}
			return
		}

		select {
		case s.frames <- frame:
		case <-s.done:
			return
		}
	}
}

func (m *AsteriskManager) send(s *session, request *actionRequest, pending map[string]*pendingAction) error {
	id := m.nextActionID()

	fields := make([]Field, 0, len(request.fields)+2)
	fields = append(fields, Field{Key: "Action", Value: request.action})
	fields = append(fields, Field{Key: "ActionID", Value: id})
	fields = append(fields, request.fields...)

	message, err := encodeAction(fields)
	if err != nil {
		// Erro de montagem é problema do chamador, a conexão continua boa.
		request.finish(nil, err)
		return nil
	}

	if err := s.conn.Send(message); err != nil {
		return err
	}

	if !request.internal {
		log.Printf("[%s] Action %s enviada (id=%s)", m.ID, request.action, id)
	}

	pending[id] = &pendingAction{
		id:       id,
		action:   request.action,
		complete: request.complete,
		ctx:      request.ctx,
		deadline: request.deadline,
		reply:    request.reply,
	}
	return nil
}

func (m *AsteriskManager) route(frame Response, pending map[string]*pendingAction, eventChan chan<- Event) bool {
	if id := frame.Get("ActionID"); id != "" {
		if action, ok := pending[id]; ok {
			done, err := action.accept(frame)
			if done {
				delete(pending, id)
				action.finish(action.frames, err)
			}
			return true
		}

		m.orphans++
		log.Printf("[%s] Frame com ActionID sem action pendente (id=%s, total=%d)", m.ID, id, m.orphans)
	}

	return m.deliverEvent(frame, eventChan)
}

func (m *AsteriskManager) deliverEvent(frame Response, eventChan chan<- Event) bool {
	event, shouldProcess := ProcessAMIEvent(frame, m.ID)
	if !shouldProcess {
		return true
	}

	select {
	case eventChan <- *event:
		return true
	case <-m.ctx.Done():
		return false
	}
}

func (m *AsteriskManager) expirePending(pending map[string]*pendingAction, now time.Time) {
	for id, action := range pending {
		if action.ctx != nil && action.ctx.Err() != nil {
			delete(pending, id)
			action.finish(nil, action.ctx.Err())
			continue
		}
		if now.After(action.deadline) {
			delete(pending, id)
			log.Printf("[%s] Timeout na action %s (id=%s)", m.ID, action.action, action.id)
			action.finish(nil, fmt.Errorf("ami: timeout na action %s", action.action))
		}
	}
}

func failPending(pending map[string]*pendingAction, reason error) {
	for id, action := range pending {
		delete(pending, id)
		action.finish(nil, reason)
	}
}

func (m *AsteriskManager) nextActionID() string {
	seq := atomic.AddUint64(&m.actionSeq, 1)
	return fmt.Sprintf("%s-%d-%d", m.ID, m.bootedAt, seq)
}

func (m *AsteriskManager) request(ctx context.Context, action string, fields []Field) (Response, error) {
	frames, err := m.submit(ctx, action, "", fields)
	if err != nil {
		return nil, err
	}
	if len(frames) == 0 {
		return nil, fmt.Errorf("ami: action %s sem resposta", action)
	}
	return frames[0], nil
}

func (m *AsteriskManager) requestList(ctx context.Context, action string, complete string, fields []Field) ([]Response, error) {
	return m.submit(ctx, action, complete, fields)
}

func (m *AsteriskManager) submit(ctx context.Context, action string, complete string, fields []Field) ([]Response, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(defaultActionTimeout)
	}

	request := &actionRequest{
		action:   action,
		fields:   fields,
		complete: complete,
		ctx:      ctx,
		deadline: deadline,
		reply:    make(chan actionResult, 1),
	}

	select {
	case m.actions <- request:
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-m.ctx.Done():
		return nil, ErrManagerClosed
	}

	select {
	case result := <-request.reply:
		return result.frames, result.err
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-m.ctx.Done():
		return nil, ErrManagerClosed
	}
}

func (m *AsteriskManager) SendAction(ctx context.Context, action map[string]string) (map[string]string, error) {
	actionType := action["Action"]
	log.Printf("[%s] Enviando action: %s", m.ID, actionType)
	return SendActionRaw(ctx, m, action)
}

func (m *AsteriskManager) Close() error {
	m.closeOnce.Do(func() {
		log.Printf("[%s] Fechando conexão AMI", m.ID)

		logoffCtx, cancel := context.WithTimeout(m.ctx, logoffTimeout)
		if _, err := m.request(logoffCtx, "Logoff", nil); err != nil {
			log.Printf("[%s] Erro no logoff: %v", m.ID, err)
		}
		cancel()

		m.cancel()
		m.wg.Wait()
	})
	return nil
}

func (m *AsteriskManager) GetWebhooks() []config.Webhook {
	return m.webhooks
}

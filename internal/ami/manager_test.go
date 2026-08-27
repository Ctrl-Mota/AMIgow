package ami

import (
	"context"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/safehouse/amigow/internal/config"
)

type serverConn struct {
	t    *testing.T
	conn net.Conn
	fr   *frameReader
}

func (s *serverConn) write(text string) bool {
	if _, err := io.WriteString(s.conn, text); err != nil {
		return false
	}
	return true
}

func (s *serverConn) readAction() (Response, bool) {
	frame, err := s.fr.ReadFrame()
	if err != nil {
		return nil, false
	}
	return frame, true
}

// autoReply responde Logoff e Ping para o teste não esperar timeout.
func (s *serverConn) autoReply(action Response) bool {
	switch action.Get("Action") {
	case "Logoff":
		return s.write("Response: Goodbye\r\nActionID: " + action.Get("ActionID") + "\r\nMessage: Thanks for all the fish.\r\n\r\n")
	case "Ping":
		return s.write("Response: Success\r\nActionID: " + action.Get("ActionID") + "\r\nPing: Pong\r\n\r\n")
	}
	return true
}

// serveIdle só mantém a conexão viva respondendo o básico.
func (s *serverConn) serveIdle() {
	for {
		action, ok := s.readAction()
		if !ok {
			return
		}
		if !s.autoReply(action) {
			return
		}
	}
}

func (s *serverConn) handshake() bool {
	if !s.write("Asterisk Call Manager/7.0.3\r\n") {
		return false
	}
	login, ok := s.readAction()
	if !ok {
		return false
	}
	if login.Get("Action") != "Login" {
		s.t.Errorf("primeira action deveria ser Login, veio %q", login.Get("Action"))
		return false
	}
	return s.write("Response: Success\r\nActionID: " + login.Get("ActionID") + "\r\nMessage: Authentication accepted\r\n\r\n")
}

type fakeAMI struct {
	listener net.Listener
	wg       sync.WaitGroup
}

func startFakeAMI(t *testing.T, script func(sc *serverConn)) *fakeAMI {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("erro ao abrir listener: %v", err)
	}

	fake := &fakeAMI{listener: listener}
	fake.wg.Add(1)
	go func() {
		defer fake.wg.Done()
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			fake.wg.Add(1)
			go func() {
				defer fake.wg.Done()
				defer conn.Close()
				script(&serverConn{t: t, conn: conn, fr: newFrameReader(conn)})
			}()
		}
	}()

	t.Cleanup(func() {
		listener.Close()
		fake.wg.Wait()
	})

	return fake
}

func (f *fakeAMI) config() config.AMIServer {
	addr := f.listener.Addr().(*net.TCPAddr)
	return config.AMIServer{
		Host:     "127.0.0.1",
		Port:     addr.Port,
		Username: "amigow",
		Password: "segredo",
	}
}

func startManager(t *testing.T, fake *fakeAMI) (*AsteriskManager, chan Event) {
	t.Helper()

	manager, err := NewAsteriskManager(context.Background(), "hx", fake.config())
	if err != nil {
		t.Fatalf("erro ao criar manager: %v", err)
	}

	eventChan := make(chan Event, 32)
	manager.Start(eventChan)
	t.Cleanup(func() { manager.Close() })

	return manager, eventChan
}

func waitEvent(t *testing.T, eventChan <-chan Event) Event {
	t.Helper()
	select {
	case event := <-eventChan:
		return event
	case <-time.After(5 * time.Second):
		t.Fatal("timeout esperando evento")
		return Event{}
	}
}

// Garante que uma action de lista não engole eventos que chegam no meio dela,
// e que os frames da lista não são entregues como evento.
func TestActionDeListaNaoEngoleEventos(t *testing.T) {
	fake := startFakeAMI(t, func(sc *serverConn) {
		if !sc.handshake() {
			return
		}
		for {
			action, ok := sc.readAction()
			if !ok {
				return
			}
			if action.Get("Action") != "QueueStatus" {
				sc.autoReply(action)
				continue
			}

			id := action.Get("ActionID")
			// Tudo em uma única escrita: ack da action, um evento que não
			// pertence a ela, um evento da lista e o terminador da lista.
			sc.write("Response: Success\r\nActionID: " + id + "\r\nMessage: Queue status will follow\r\n\r\n" +
				"Event: QueueCallerJoin\r\nQueue: 7000\r\nUniqueid: 1.1\r\nPosition: 1\r\n\r\n" +
				"Event: QueueParams\r\nActionID: " + id + "\r\nQueue: 7000\r\nStrategy: ringall\r\n\r\n" +
				"Event: QueueStatusComplete\r\nActionID: " + id + "\r\n\r\n")
		}
	})

	manager, eventChan := startManager(t, fake)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	frames, err := SendQueueStatusAll(ctx, manager)
	if err != nil {
		t.Fatalf("erro no QueueStatus: %v", err)
	}
	if len(frames) != 1 {
		t.Fatalf("esperava 1 frame na lista, veio %d: %v", len(frames), frames)
	}
	if frames[0].Get("Event") != "QueueParams" {
		t.Fatalf("frame da lista inesperado: %v", frames[0])
	}

	event := waitEvent(t, eventChan)
	if event.Type != "queue_join" {
		t.Fatalf("evento deveria ser queue_join, veio %q", event.Type)
	}
	if event.Data["Queue"] != "7000" {
		t.Fatalf("evento sem a queue correta: %v", event.Data)
	}

	select {
	case extra := <-eventChan:
		t.Fatalf("frame da action virou evento: %v", extra.Data)
	case <-time.After(200 * time.Millisecond):
	}
}

// Actions simultâneas não podem trocar de resposta entre si.
func TestActionsSimultaneasNaoTrocamRespostas(t *testing.T) {
	fake := startFakeAMI(t, func(sc *serverConn) {
		if !sc.handshake() {
			return
		}
		for {
			action, ok := sc.readAction()
			if !ok {
				return
			}
			if action.Get("Action") != "Getvar" {
				sc.autoReply(action)
				continue
			}
			// Responde na ordem inversa do pedido, com um evento no meio.
			time.Sleep(20 * time.Millisecond)
			sc.write("Event: Newchannel\r\nChannelStateDesc: Ring\r\nChannel: PJSIP/" + action.Get("Variable") + "\r\n\r\n")
			sc.write("Response: Success\r\nActionID: " + action.Get("ActionID") +
				"\r\nVariable: " + action.Get("Variable") +
				"\r\nValue: valor-" + action.Get("Variable") + "\r\n\r\n")
		}
	})

	manager, eventChan := startManager(t, fake)

	go func() {
		for range eventChan {
		}
	}()

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			name := "VAR" + string(rune('A'+index))
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			response, err := GetChannelVar(ctx, manager, "PJSIP/100", name)
			if err != nil {
				t.Errorf("erro no Getvar %s: %v", name, err)
				return
			}
			if response.Get("Value") != "valor-"+name {
				t.Errorf("resposta trocada: pedi %s e recebi %q", name, response.Get("Value"))
			}
		}(i)
	}
	wg.Wait()
}

func TestActionRespeitaTimeoutDoContexto(t *testing.T) {
	fake := startFakeAMI(t, func(sc *serverConn) {
		if !sc.handshake() {
			return
		}
		sc.serveIdle()
	})

	manager, eventChan := startManager(t, fake)
	go func() {
		for range eventChan {
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	start := time.Now()
	if _, err := GetCoreStatus(ctx, manager); err == nil {
		t.Fatal("esperava erro de timeout")
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("timeout demorou demais: %s", elapsed)
	}
}

func TestReconectaEContinuaEntregandoEventos(t *testing.T) {
	var connections atomic.Int32

	fake := startFakeAMI(t, func(sc *serverConn) {
		attempt := connections.Add(1)
		if !sc.handshake() {
			return
		}
		if attempt == 1 {
			// Derruba a primeira conexão logo depois do login.
			sc.conn.Close()
			return
		}
		sc.write("Event: Hangup\r\nChannel: PJSIP/101\r\nUniqueid: 2.2\r\nCause: 16\r\n\r\n")
		sc.serveIdle()
	})

	_, eventChan := startManager(t, fake)

	event := waitEvent(t, eventChan)
	if event.Type != "hangup" {
		t.Fatalf("evento deveria ser hangup, veio %q", event.Type)
	}
	if connections.Load() < 2 {
		t.Fatalf("esperava reconexão, houve %d conexão", connections.Load())
	}
}

func TestActionFalhaQuandoConexaoCai(t *testing.T) {
	fake := startFakeAMI(t, func(sc *serverConn) {
		if !sc.handshake() {
			return
		}
		action, ok := sc.readAction()
		if !ok {
			return
		}
		if action.Get("Action") == "CoreStatus" {
			sc.conn.Close()
		}
	})

	manager, eventChan := startManager(t, fake)
	go func() {
		for range eventChan {
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := GetCoreStatus(ctx, manager); err == nil {
		t.Fatal("esperava erro quando a conexão cai no meio da action")
	}
}

func TestCloseLiberaActionPendente(t *testing.T) {
	fake := startFakeAMI(t, func(sc *serverConn) {
		if !sc.handshake() {
			return
		}
		sc.serveIdle()
	})

	manager, eventChan := startManager(t, fake)
	go func() {
		for range eventChan {
		}
	}()

	failed := make(chan error, 1)
	go func() {
		_, err := GetCoreStatus(context.Background(), manager)
		failed <- err
	}()

	time.Sleep(200 * time.Millisecond)

	closed := make(chan struct{})
	go func() {
		manager.Close()
		close(closed)
	}()

	select {
	case err := <-failed:
		if err == nil {
			t.Fatal("action pendente deveria falhar no shutdown")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("action pendente ficou presa no shutdown")
	}

	select {
	case <-closed:
	case <-time.After(10 * time.Second):
		t.Fatal("Close não terminou")
	}
}

func TestLoginRecusadoDevolveErro(t *testing.T) {
	fake := startFakeAMI(t, func(sc *serverConn) {
		if !sc.write("Asterisk Call Manager/7.0.3\r\n") {
			return
		}
		login, ok := sc.readAction()
		if !ok {
			return
		}
		sc.write("Response: Error\r\nActionID: " + login.Get("ActionID") + "\r\nMessage: Authentication failed\r\n\r\n")
	})

	if _, err := NewAsteriskManager(context.Background(), "hx", fake.config()); err == nil {
		t.Fatal("esperava erro de login recusado")
	}
}

func TestSaudacaoInvalidaDevolveErro(t *testing.T) {
	fake := startFakeAMI(t, func(sc *serverConn) {
		sc.write("SSH-2.0-OpenSSH\r\n")
	})

	if _, err := NewAsteriskManager(context.Background(), "hx", fake.config()); err == nil {
		t.Fatal("esperava erro de saudação inválida")
	}
}

package ami

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

// chunkReader entrega os bytes em pedaços controlados, imitando o
// fatiamento que o TCP faz com as mensagens do AMI.
type chunkReader struct {
	chunks [][]byte
	err    error
}

func (c *chunkReader) Read(p []byte) (int, error) {
	if len(c.chunks) == 0 {
		if c.err != nil {
			return 0, c.err
		}
		return 0, io.EOF
	}

	chunk := c.chunks[0]
	c.chunks = c.chunks[1:]
	n := copy(p, chunk)
	if n < len(chunk) {
		c.chunks = append([][]byte{chunk[n:]}, c.chunks...)
	}
	return n, nil
}

type dataWithErrorReader struct {
	data []byte
	sent bool
	err  error
}

func (r *dataWithErrorReader) Read(p []byte) (int, error) {
	if r.sent {
		return 0, r.err
	}
	r.sent = true
	return copy(p, r.data), r.err
}

func chunks(parts ...string) *chunkReader {
	reader := &chunkReader{}
	for _, part := range parts {
		reader.chunks = append(reader.chunks, []byte(part))
	}
	return reader
}

func TestReadFrameEntregaDoisEventosDeUmaLeitura(t *testing.T) {
	stream := "Event: Newstate\r\nChannel: PJSIP/101\r\n\r\n" +
		"Event: QueueCallerJoin\r\nQueue: 7000\r\n\r\n"
	reader := newFrameReader(chunks(stream))

	first, err := reader.ReadFrame()
	if err != nil {
		t.Fatalf("erro no primeiro frame: %v", err)
	}
	if first.Get("Event") != "Newstate" {
		t.Fatalf("primeiro frame deveria ser Newstate, veio %q", first.Get("Event"))
	}

	second, err := reader.ReadFrame()
	if err != nil {
		t.Fatalf("erro no segundo frame: %v", err)
	}
	if second.Get("Event") != "QueueCallerJoin" {
		t.Fatalf("segundo frame deveria ser QueueCallerJoin, veio %q", second.Get("Event"))
	}
	if second.Get("Queue") != "7000" {
		t.Fatalf("Queue deveria ser 7000, veio %q", second.Get("Queue"))
	}
}

func TestReadFrameGuardaEventoParcialParaProximaLeitura(t *testing.T) {
	reader := newFrameReader(chunks(
		"Event: Newstate\r\nChannel: PJSIP/101\r\n\r\nEvent: QueueCallerJ",
		"oin\r\nQueue: 7000\r\n\r\n",
	))

	first, err := reader.ReadFrame()
	if err != nil {
		t.Fatalf("erro no primeiro frame: %v", err)
	}
	if first.Get("Event") != "Newstate" {
		t.Fatalf("primeiro frame deveria ser Newstate, veio %q", first.Get("Event"))
	}

	second, err := reader.ReadFrame()
	if err != nil {
		t.Fatalf("erro no segundo frame: %v", err)
	}
	if second.Get("Event") != "QueueCallerJoin" {
		t.Fatalf("segundo frame deveria ser QueueCallerJoin, veio %q", second.Get("Event"))
	}
}

func TestReadFrameComTerminadorPartidoEntreLeituras(t *testing.T) {
	reader := newFrameReader(chunks(
		"Event: Hangup\r\nChannel: PJSIP/101\r\n\r",
		"\nEvent: DTMFEnd\r\nDigit: 5\r\n\r\n",
	))

	first, err := reader.ReadFrame()
	if err != nil {
		t.Fatalf("erro no primeiro frame: %v", err)
	}
	if first.Get("Event") != "Hangup" {
		t.Fatalf("primeiro frame deveria ser Hangup, veio %q", first.Get("Event"))
	}

	second, err := reader.ReadFrame()
	if err != nil {
		t.Fatalf("erro no segundo frame: %v", err)
	}
	if second.Get("Digit") != "5" {
		t.Fatalf("Digit deveria ser 5, veio %q", second.Get("Digit"))
	}
}

func TestReadFrameMantemValorComDoisPontos(t *testing.T) {
	reader := newFrameReader(chunks("Event: VarSet\r\nValue: sip:100@10.0.0.1:5060\r\n\r\n"))

	frame, err := reader.ReadFrame()
	if err != nil {
		t.Fatalf("erro no frame: %v", err)
	}
	if frame.Get("Value") != "sip:100@10.0.0.1:5060" {
		t.Fatalf("valor com dois pontos foi cortado: %q", frame.Get("Value"))
	}
}

func TestReadFrameGuardaChavesRepetidas(t *testing.T) {
	reader := newFrameReader(chunks("Response: Success\r\nOutput: linha1\r\nOutput: linha2\r\n\r\n"))

	frame, err := reader.ReadFrame()
	if err != nil {
		t.Fatalf("erro no frame: %v", err)
	}
	values := frame.GetAll("Output")
	if len(values) != 2 || values[0] != "linha1" || values[1] != "linha2" {
		t.Fatalf("chaves repetidas perdidas: %v", values)
	}
	if frame.Get("Output") != "linha1" {
		t.Fatalf("Get deveria devolver o primeiro valor, veio %q", frame.Get("Output"))
	}
}

func TestReadFrameEntregaFrameCompletoAntesDoErro(t *testing.T) {
	reader := newFrameReader(&dataWithErrorReader{
		data: []byte("Event: Hangup\r\nChannel: PJSIP/101\r\n\r\n"),
		err:  io.EOF,
	})

	frame, err := reader.ReadFrame()
	if err != nil {
		t.Fatalf("frame completo deveria vir antes do erro: %v", err)
	}
	if frame.Get("Event") != "Hangup" {
		t.Fatalf("frame deveria ser Hangup, veio %q", frame.Get("Event"))
	}

	if _, err := reader.ReadFrame(); !errors.Is(err, io.EOF) {
		t.Fatalf("segunda leitura deveria devolver EOF, veio %v", err)
	}
}

func TestReadFrameDescartaFrameParcialNoEOF(t *testing.T) {
	reader := newFrameReader(chunks("Event: Hangup\r\nChannel: PJSIP"))

	if _, err := reader.ReadFrame(); !errors.Is(err, io.EOF) {
		t.Fatalf("esperava EOF, veio %v", err)
	}
}

func TestReadFrameRecusaMensagemMaiorQueOLimite(t *testing.T) {
	huge := string(bytes.Repeat([]byte("a"), maxFrameBytes+1024))
	reader := newFrameReader(chunks(huge))

	_, err := reader.ReadFrame()
	if err == nil {
		t.Fatal("esperava erro de mensagem sem terminador")
	}
	if !strings.Contains(err.Error(), "sem terminador") {
		t.Fatalf("erro inesperado: %v", err)
	}
}

func TestReadLineLeSaudacaoDoAsterisk(t *testing.T) {
	reader := newFrameReader(chunks("Asterisk Call Manager/7.0.3\r\nEvent: Hangup\r\n\r\n"))

	greeting, err := reader.ReadLine()
	if err != nil {
		t.Fatalf("erro ao ler saudação: %v", err)
	}
	if greeting != "Asterisk Call Manager/7.0.3" {
		t.Fatalf("saudação inesperada: %q", greeting)
	}

	frame, err := reader.ReadFrame()
	if err != nil {
		t.Fatalf("erro ao ler frame após saudação: %v", err)
	}
	if frame.Get("Event") != "Hangup" {
		t.Fatalf("frame após saudação deveria ser Hangup, veio %q", frame.Get("Event"))
	}
}

func TestEncodeActionMontaMensagemAMI(t *testing.T) {
	message, err := encodeAction([]Field{
		{Key: "Action", Value: "QueueStatus"},
		{Key: "ActionID", Value: "hx-1"},
		{Key: "Queue", Value: "7000"},
	})
	if err != nil {
		t.Fatalf("erro ao montar action: %v", err)
	}

	want := "Action: QueueStatus\r\nActionID: hx-1\r\nQueue: 7000\r\n\r\n"
	if message != want {
		t.Fatalf("mensagem inesperada: %q", message)
	}
}

func TestEncodeActionRecusaQuebraDeLinha(t *testing.T) {
	if _, err := encodeAction([]Field{{Key: "Command", Value: "core show channels\r\nAction: Logoff"}}); err == nil {
		t.Fatal("esperava erro por quebra de linha no valor")
	}
	if _, err := encodeAction([]Field{{Key: "Bad:Key", Value: "x"}}); err == nil {
		t.Fatal("esperava erro por dois pontos no nome do campo")
	}
}

func TestAppendFieldIgnoraValorVazio(t *testing.T) {
	fields := appendField(nil, "Queue", "")
	fields = appendField(fields, "Interface", "PJSIP/100")

	if len(fields) != 1 || fields[0].Key != "Interface" {
		t.Fatalf("campos inesperados: %v", fields)
	}
}

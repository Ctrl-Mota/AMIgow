package ami

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
)

const (
	maxFrameBytes  = 1 << 20
	readChunkBytes = 8 * 1024
	shrinkBufBytes = 64 * 1024
)

var (
	frameDelimiter = []byte("\r\n\r\n")
	lineDelimiter  = []byte("\r\n")
)

// Response guarda os campos de uma mensagem AMI. A mesma chave pode aparecer
// mais de uma vez, por isso cada chave guarda uma lista de valores.
type Response map[string][]string

func (r Response) Get(key string) string {
	values := r[key]
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func (r Response) GetAll(key string) []string {
	return r[key]
}

type Field struct {
	Key   string
	Value string
}

func appendField(fields []Field, key string, value string) []Field {
	if value == "" {
		return fields
	}
	return append(fields, Field{Key: key, Value: value})
}

func encodeAction(fields []Field) (string, error) {
	var out strings.Builder
	for _, field := range fields {
		if field.Key == "" {
			return "", errors.New("ami: campo sem nome")
		}
		if strings.ContainsAny(field.Key, "\r\n:") {
			return "", fmt.Errorf("ami: nome de campo inválido: %q", field.Key)
		}
		if strings.ContainsAny(field.Value, "\r\n") {
			return "", fmt.Errorf("ami: valor inválido no campo %s", field.Key)
		}
		out.WriteString(field.Key)
		out.WriteString(": ")
		out.WriteString(field.Value)
		out.WriteString("\r\n")
	}
	out.WriteString("\r\n")
	return out.String(), nil
}

// frameReader guarda os bytes que sobraram entre leituras. Sem esse buffer
// persistente, dois eventos que chegam juntos no mesmo pacote TCP fariam o
// segundo evento ser descartado.
type frameReader struct {
	src        io.Reader
	buf        []byte
	chunk      []byte
	pendingErr error
}

func newFrameReader(src io.Reader) *frameReader {
	return &frameReader{
		src:   src,
		buf:   make([]byte, 0, readChunkBytes),
		chunk: make([]byte, readChunkBytes),
	}
}

func (fr *frameReader) ReadFrame() (Response, error) {
	for {
		if frame, ok := fr.take(frameDelimiter); ok {
			return parseFrame(frame), nil
		}
		if err := fr.fill(); err != nil {
			return nil, err
		}
	}
}

func (fr *frameReader) ReadLine() (string, error) {
	for {
		if line, ok := fr.take(lineDelimiter); ok {
			return string(line), nil
		}
		if err := fr.fill(); err != nil {
			return "", err
		}
	}
}

func (fr *frameReader) fill() error {
	if fr.pendingErr != nil {
		return fr.pendingErr
	}
	if len(fr.buf) > maxFrameBytes {
		fr.pendingErr = fmt.Errorf("ami: mensagem passou de %d bytes sem terminador", maxFrameBytes)
		return fr.pendingErr
	}

	n, err := fr.src.Read(fr.chunk)
	if n > 0 {
		fr.buf = append(fr.buf, fr.chunk[:n]...)
	}
	if err != nil {
		fr.pendingErr = err
		// Se chegaram bytes junto com o erro, o chamador ainda pode ter uma
		// mensagem completa para consumir antes de ver o erro.
		if n > 0 {
			return nil
		}
		return err
	}
	return nil
}

func (fr *frameReader) take(delimiter []byte) ([]byte, bool) {
	index := bytes.Index(fr.buf, delimiter)
	if index < 0 {
		return nil, false
	}

	frame := make([]byte, index)
	copy(frame, fr.buf[:index])

	rest := fr.buf[index+len(delimiter):]
	copy(fr.buf, rest)
	fr.buf = fr.buf[:len(rest)]

	if len(fr.buf) == 0 && cap(fr.buf) > shrinkBufBytes {
		fr.buf = make([]byte, 0, readChunkBytes)
	}

	return frame, true
}

func parseFrame(frame []byte) Response {
	response := make(Response)
	for _, line := range strings.Split(string(frame), "\r\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		if key == "" {
			continue
		}
		response[key] = append(response[key], strings.TrimSpace(parts[1]))
	}
	return response
}

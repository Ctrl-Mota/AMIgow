package ami

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	goami "github.com/heltonmarx/goami/ami"
)

type LoggedSocket struct {
	*goami.Socket
	logFile *os.File
	id      string
}

func NewLoggedSocket(ctx context.Context, address string, logFile *os.File, id string) (*LoggedSocket, error) {
	socket, err := goami.NewSocket(ctx, address)
	if err != nil {
		return nil, err
	}

	return &LoggedSocket{
		Socket:  socket,
		logFile: logFile,
		id:      id,
	}, nil
}

func (ls *LoggedSocket) Send(message string) error {
	log.Printf("[%s] Enviando mensagem: %s", ls.id, message)
	if ls.logFile != nil {
		timestamp := time.Now().Format("2006-01-02 15:04:05.000")
		fmt.Fprintf(ls.logFile, "\n<<< SEND [%s]\n", timestamp)
		fmt.Fprintf(ls.logFile, "%s", message)
		ls.logFile.Sync()
	}

	return ls.Socket.Send(message)
}

func (ls *LoggedSocket) Recv(ctx context.Context) (string, error) {
	data, err := ls.Socket.Recv(ctx)
	//log.Printf("[%s] Recebendo mensagem: %s", ls.id, data)
	// if ls.logFile != nil && err == nil && data != "" {
	// 	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	// 	fmt.Fprintf(ls.logFile, "\n>>> RECV [%s]\n", timestamp)
	// 	fmt.Fprintf(ls.logFile, "%s", data)
	// 	ls.logFile.Sync()
	// }
	// //escreva o erro recebido também no log file
	// if err != nil {
	// 	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	// 	fmt.Fprintf(ls.logFile, "\n>>> ERROR [%s]\n", timestamp)
	// 	fmt.Fprintf(ls.logFile, "%s", data)
	// 	ls.logFile.Sync()
	// }

	return data, err
}

func (ls *LoggedSocket) Close(ctx context.Context) error {
	return ls.Socket.Close(ctx)
}

func (ls *LoggedSocket) Connected() bool {
	return ls.Socket.Connected()
}

package ami

import (
	"context"

	goami "github.com/heltonmarx/goami/ami"
)

type QueueData struct {
	Queue     string
	Interface string
}

func SendQueueAdd(ctx context.Context, mgr *AsteriskManager, data QueueData) (Response, error) {
	queueData := goami.QueueData{
		Queue:     "Q" + data.Queue,
		Interface: "PJSIP/" + data.Interface,
		Penalty:   "0",
		Paused:    "false",
	}
	return goami.QueueAdd(ctx, mgr.socket, mgr.uuid, queueData)
}

func SendQueueRemove(ctx context.Context, mgr *AsteriskManager, queue string, iface string) (Response, error) {
	queueData := goami.QueueData{
		Queue:     "Q" + queue,
		Interface: "PJSIP/" + iface,
	}
	return goami.QueueRemove(ctx, mgr.socket, mgr.uuid, queueData)
}

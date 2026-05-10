package ami

import (
	"context"

	goami "github.com/heltonmarx/goami/ami"
)

type ChannelData struct {
	Channel  string
	Exten    string
	Context  string
	Priority string
}

func SendChannelRedirect(ctx context.Context, mgr *AsteriskManager, data ChannelData) (Response, error) {
	callData := goami.CallData{
		Channel:  data.Channel,
		Exten:    data.Exten,
		Context:  data.Context,
		Priority: data.Priority,
	}
	return goami.Redirect(ctx, mgr.socket, mgr.uuid, callData)
}

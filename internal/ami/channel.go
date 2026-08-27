package ami

import "context"

type ChannelData struct {
	Channel  string
	Exten    string
	Context  string
	Priority string
}

func SendChannelRedirect(ctx context.Context, mgr *AsteriskManager, data ChannelData) (Response, error) {
	return mgr.request(ctx, "Redirect", []Field{
		{Key: "Channel", Value: data.Channel},
		{Key: "Exten", Value: data.Exten},
		{Key: "Context", Value: data.Context},
		{Key: "Priority", Value: data.Priority},
	})
}

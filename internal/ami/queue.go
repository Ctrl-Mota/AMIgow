package ami

import "context"

const queueStatusComplete = "QueueStatusComplete"

type QueueData struct {
	Queue     string
	Interface string
}

func SendQueueAdd(ctx context.Context, mgr *AsteriskManager, data QueueData) (Response, error) {
	return mgr.request(ctx, "QueueAdd", []Field{
		{Key: "Queue", Value: data.Queue},
		{Key: "Interface", Value: "PJSIP/" + data.Interface},
		{Key: "Penalty", Value: "0"},
		{Key: "Paused", Value: "false"},
	})
}

func SendQueueRemove(ctx context.Context, mgr *AsteriskManager, queue string, iface string) (Response, error) {
	return mgr.request(ctx, "QueueRemove", []Field{
		{Key: "Queue", Value: queue},
		{Key: "Interface", Value: "PJSIP/" + iface},
	})
}

func SendQueueMemberStatus(ctx context.Context, mgr *AsteriskManager, queue string, iface string) ([]Response, error) {
	return mgr.requestList(ctx, "QueueStatus", queueStatusComplete, []Field{
		{Key: "Queue", Value: queue},
		{Key: "Member", Value: "PJSIP/" + iface},
	})
}

func SendQueueStatus(ctx context.Context, mgr *AsteriskManager, queue string) ([]Response, error) {
	return mgr.requestList(ctx, "QueueStatus", queueStatusComplete, []Field{
		{Key: "Queue", Value: queue},
	})
}

func SendQueueStatusAll(ctx context.Context, mgr *AsteriskManager) ([]Response, error) {
	return mgr.requestList(ctx, "QueueStatus", queueStatusComplete, nil)
}

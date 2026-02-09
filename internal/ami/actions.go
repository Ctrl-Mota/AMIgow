package ami

import (
	"context"
	"fmt"

	goami "github.com/heltonmarx/goami/ami"
)

type Response interface {
	Get(key string) string
}

type OriginateData struct {
	Channel     string
	Exten       string
	Context     string
	Priority    int
	CallerID    string
	Timeout     int
	Variable    []string
	Application string
	Data        string
	Async       string
}

func SendOriginate(ctx context.Context, mgr *AsteriskManager, data OriginateData) (Response, error) {
	originateData := goami.OriginateData{
		Channel:     data.Channel,
		Exten:       data.Exten,
		Context:     data.Context,
		Priority:    data.Priority,
		CallerID:    data.CallerID,
		Timeout:     data.Timeout,
		Variable:    data.Variable,
		Application: data.Application,
		Data:        data.Data,
		Async:       data.Async,
	}

	return goami.Originate(ctx, mgr.socket, mgr.uuid, originateData)
}

func SendHangup(ctx context.Context, mgr *AsteriskManager, channel string, cause string) (Response, error) {
	if cause == "" {
		cause = "16"
	}
	return goami.Hangup(ctx, mgr.socket, mgr.uuid, channel, cause)
}

func SendCommand(ctx context.Context, mgr *AsteriskManager, command string) (Response, error) {
	return goami.Command(ctx, mgr.socket, mgr.uuid, command)
}

func SendPing(ctx context.Context, mgr *AsteriskManager) error {
	return goami.Ping(ctx, mgr.socket, mgr.uuid)
}

func GetCoreStatus(ctx context.Context, mgr *AsteriskManager) (Response, error) {
	return goami.CoreStatus(ctx, mgr.socket, mgr.uuid)
}

func GetCoreShowChannels(ctx context.Context, mgr *AsteriskManager) ([]Response, error) {
	channels, err := goami.CoreShowChannels(ctx, mgr.socket, mgr.uuid)
	if err != nil {
		return nil, err
	}
	result := make([]Response, len(channels))
	for i, ch := range channels {
		result[i] = ch
	}
	return result, nil
}

func GetChannelStatus(ctx context.Context, mgr *AsteriskManager, channel string) (Response, error) {
	return goami.Status(ctx, mgr.socket, mgr.uuid, channel, "")
}

func SetChannelVar(ctx context.Context, mgr *AsteriskManager, channel string, variable string, value string) (Response, error) {
	return goami.Setvar(ctx, mgr.socket, mgr.uuid, channel, variable, value)
}

func GetChannelVar(ctx context.Context, mgr *AsteriskManager, channel string, variable string) (Response, error) {
	return goami.Getvar(ctx, mgr.socket, mgr.uuid, channel, variable)
}

func SendRedirect(ctx context.Context, mgr *AsteriskManager, channel string, exten string, context string, priority string) (Response, error) {
	callData := goami.CallData{
		Channel:  channel,
		Exten:    exten,
		Context:  context,
		Priority: priority,
	}
	return goami.Redirect(ctx, mgr.socket, mgr.uuid, callData)
}

func SendBridge(ctx context.Context, mgr *AsteriskManager, channel1 string, channel2 string, tone string) (Response, error) {
	if tone == "" {
		tone = "no"
	}
	return goami.Bridge(ctx, mgr.socket, mgr.uuid, channel1, channel2, tone)
}

func SendActionRaw(ctx context.Context, mgr *AsteriskManager, action map[string]string) (map[string]string, error) {
	actionType := action["Action"]
	if actionType == "" {
		return nil, fmt.Errorf("campo Action é obrigatório")
	}

	response := make(map[string]string)

	switch actionType {
	case "Ping":
		err := SendPing(ctx, mgr)
		if err != nil {
			return nil, err
		}

	case "Command":
		command := action["Command"]
		if command == "" {
			return nil, fmt.Errorf("campo Command é obrigatório")
		}
		resp, err := SendCommand(ctx, mgr, command)
		if err != nil {
			return nil, err
		}
		response["Response"] = resp.Get("Response")
		response["Output"] = resp.Get("Output")

	case "Hangup":
		channel := action["Channel"]
		if channel == "" {
			return nil, fmt.Errorf("campo Channel é obrigatório")
		}
		cause := action["Cause"]
		resp, err := SendHangup(ctx, mgr, channel, cause)
		if err != nil {
			return nil, err
		}
		response["Response"] = resp.Get("Response")
		response["Message"] = resp.Get("Message")

	case "Status":
		channel := action["Channel"]
		resp, err := GetChannelStatus(ctx, mgr, channel)
		if err != nil {
			return nil, err
		}
		response["Response"] = resp.Get("Response")
		response["Channel"] = resp.Get("Channel")
		response["ChannelState"] = resp.Get("ChannelState")
		response["ChannelStateDesc"] = resp.Get("ChannelStateDesc")

	case "CoreStatus":
		resp, err := GetCoreStatus(ctx, mgr)
		if err != nil {
			return nil, err
		}
		response["Response"] = resp.Get("Response")
		response["CoreCurrentCalls"] = resp.Get("CoreCurrentCalls")
		response["CoreMaxCalls"] = resp.Get("CoreMaxCalls")

	default:
		return nil, fmt.Errorf("action %s não implementada", actionType)
	}

	return response, nil
}

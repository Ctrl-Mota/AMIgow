package ami

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

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
	fields := make([]Field, 0, 10+len(data.Variable))
	fields = appendField(fields, "Channel", data.Channel)
	fields = appendField(fields, "Exten", data.Exten)
	fields = appendField(fields, "Context", data.Context)
	if data.Priority > 0 {
		fields = appendField(fields, "Priority", strconv.Itoa(data.Priority))
	}
	fields = appendField(fields, "Application", data.Application)
	fields = appendField(fields, "Data", data.Data)
	if data.Timeout > 0 {
		fields = appendField(fields, "Timeout", strconv.Itoa(data.Timeout))
	}
	fields = appendField(fields, "CallerID", data.CallerID)
	for _, variable := range data.Variable {
		fields = appendField(fields, "Variable", variable)
	}
	fields = appendField(fields, "Async", data.Async)

	return mgr.request(ctx, "Originate", fields)
}

func SendHangup(ctx context.Context, mgr *AsteriskManager, channel string, cause string) (Response, error) {
	if cause == "" {
		cause = "16"
	}
	return mgr.request(ctx, "Hangup", []Field{
		{Key: "Channel", Value: channel},
		{Key: "Cause", Value: cause},
	})
}

func SendCommand(ctx context.Context, mgr *AsteriskManager, command string) (Response, error) {
	return mgr.request(ctx, "Command", []Field{
		{Key: "Command", Value: command},
	})
}

func SendPing(ctx context.Context, mgr *AsteriskManager) error {
	response, err := mgr.request(ctx, "Ping", nil)
	if err != nil {
		return err
	}
	if !strings.EqualFold(response.Get("Response"), "Success") {
		return fmt.Errorf("ping falhou: %s", response.Get("Message"))
	}
	return nil
}

func GetCoreStatus(ctx context.Context, mgr *AsteriskManager) (Response, error) {
	return mgr.request(ctx, "CoreStatus", nil)
}

func GetCoreShowChannels(ctx context.Context, mgr *AsteriskManager) ([]Response, error) {
	return mgr.requestList(ctx, "CoreShowChannels", "CoreShowChannelsComplete", nil)
}

func GetChannelStatus(ctx context.Context, mgr *AsteriskManager, channel string) (Response, error) {
	return mgr.request(ctx, "Status", []Field{
		{Key: "Channel", Value: channel},
	})
}

func SetChannelVar(ctx context.Context, mgr *AsteriskManager, channel string, variable string, value string) (Response, error) {
	return mgr.request(ctx, "Setvar", []Field{
		{Key: "Channel", Value: channel},
		{Key: "Variable", Value: variable},
		{Key: "Value", Value: value},
	})
}

func GetChannelVar(ctx context.Context, mgr *AsteriskManager, channel string, variable string) (Response, error) {
	return mgr.request(ctx, "Getvar", []Field{
		{Key: "Channel", Value: channel},
		{Key: "Variable", Value: variable},
	})
}

func SendRedirect(ctx context.Context, mgr *AsteriskManager, channel string, exten string, context string, priority string) (Response, error) {
	return mgr.request(ctx, "Redirect", []Field{
		{Key: "Channel", Value: channel},
		{Key: "Exten", Value: exten},
		{Key: "Context", Value: context},
		{Key: "Priority", Value: priority},
	})
}

func SendBridge(ctx context.Context, mgr *AsteriskManager, channel1 string, channel2 string, tone string) (Response, error) {
	if tone == "" {
		tone = "no"
	}
	return mgr.request(ctx, "Bridge", []Field{
		{Key: "Channel1", Value: channel1},
		{Key: "Channel2", Value: channel2},
		{Key: "Tone", Value: tone},
	})
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

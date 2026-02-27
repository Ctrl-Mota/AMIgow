package api

type ActionRequest struct {
	Action map[string]string `json:"action" doc:"Ação AMI a ser executada"`
}

type ActionResponse struct {
	Body struct {
		Response map[string]string `json:"response" doc:"Resposta do servidor AMI"`
	}
}

type QueueRequest struct {
	Queue     string `json:"queue" doc:"Nome da fila" example:"7000"`
	Interface string `json:"interface" doc:"Interface do agente" example:"8001"`
}

type QueueResponse struct {
	Body struct {
		Response string `json:"Response" doc:"Status da resposta" example:"Success"`
		Message  string `json:"Message" doc:"Mensagem de retorno" example:"Added interface to queue"`
	}
}
type QueueResponseStatus struct {
	Body struct {
		Response                   string                     `json:"Response" doc:"Status da resposta" example:"Success"`
		Message                    string                     `json:"Message" doc:"Mensagem de retorno" example:"Added interface to queue"`
		QueueResponseStatusPayload QueueResponseStatusPayload `json:"payload" doc:"Payload do evento"`
	}
}

type QueueResponseStatusPayload struct {
	Max               string `json:"Max" doc:"Máximo de agentes na fila" example:"10"`
	Strategy          string `json:"Strategy" doc:"Estratégia de atendimento" example:"roundrobin"`
	Calls             string `json:"Calls" doc:"Total de chamadas na fila" example:"10"`
	Holdtime          string `json:"Holdtime" doc:"Tempo de espera na fila" example:"10"`
	TalkTime          string `json:"TalkTime" doc:"Tempo de fala na fila" example:"10"`
	Completed         string `json:"Completed" doc:"Total de chamadas concluídas" example:"10"`
	Abandoned         string `json:"Abandoned" doc:"Total de chamadas abandonadas" example:"10"`
	ServiceLevelPerf  string `json:"ServiceLevelPerf" doc:"Performance de atendimento" example:"10"`
	ServiceLevelPerf2 string `json:"ServiceLevelPerf2" doc:"Performance de atendimento 2" example:"10"`
}

type HealthResponse struct {
	Body struct {
		Status   string `json:"status" doc:"Status do serviço" example:"ok"`
		Managers int    `json:"managers" doc:"Número de managers AMI conectados" example:"2"`
	}
}

type ErrorResponse struct {
	Error string `json:"error" doc:"Mensagem de erro"`
}

type WebhookCallbackPayload struct {
	EventType   string            `json:"event_type" doc:"Tipo do evento AMI" example:"answer"`
	Source      string            `json:"source" doc:"ID do servidor Asterisk origem" example:"teste-1"`
	Timestamp   string            `json:"timestamp" doc:"Data/hora do evento em RFC3339" example:"2026-02-10T17:00:00Z"`
	Channel     string            `json:"channel,omitempty" doc:"Canal SIP/PJSIP do evento" example:"SIP/1001-0000001"`
	CallerID    string            `json:"caller_id,omitempty" doc:"Número do originador" example:"1001"`
	CallerName  string            `json:"caller_name,omitempty" doc:"Nome do originador" example:"João Silva"`
	Destination string            `json:"destination,omitempty" doc:"Número de destino/ramal" example:"2000"`
	Cause       string            `json:"cause,omitempty" doc:"Código da causa do evento" example:"16"`
	CauseText   string            `json:"cause_text,omitempty" doc:"Texto descritivo da causa" example:"Normal clearing"`
	Duration    string            `json:"duration,omitempty" doc:"Duração em segundos" example:"45"`
	RawData     map[string]string `json:"raw_data" doc:"Dados brutos do evento AMI"`
}

type WebhookSchemaResponse struct {
	Body struct {
		Description string                 `json:"description" doc:"Descrição dos webhooks enviados"`
		Payload     WebhookCallbackPayload `json:"payload_example" doc:"Exemplo de payload enviado"`
		Headers     map[string]string      `json:"headers" doc:"Headers HTTP enviados no webhook"`
		Behavior    WebhookBehavior        `json:"behavior" doc:"Comportamento de retry e timeout"`
	}
}

type WebhookBehavior struct {
	Timeout     string `json:"timeout" doc:"Timeout configurável por webhook" example:"10s"`
	Retry       string `json:"retry" doc:"Estratégia de retry" example:"1 tentativa após 1 segundo"`
	FilterLogic string `json:"filter_logic" doc:"Como funciona o filtro de eventos" example:"Apenas eventos listados em events_filter são enviados"`
}

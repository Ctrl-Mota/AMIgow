package api

import "encoding/json"

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
type QueueStatusRequest struct {
	Queue      string `json:"queue" doc:"Nome da fila" example:"7000"`
	Interfaces string `json:"interfaces" doc:"Interfaces dos agentes" example:"8001,8002"`
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

type ChannelRedirectRequest struct {
	Channel  string `json:"channel"  doc:"Canal a ser redirecionado" example:"PJSIP/8001-00000001"`
	Exten    string `json:"exten"    doc:"Ramal de destino" example:"2000"`
	Context  string `json:"context"  doc:"Contexto do dialplan" example:"from-internal"`
	Priority string `json:"priority" doc:"Prioridade no dialplan" example:"1"`
}

type ChannelRedirectResponse struct {
	Body struct {
		Response string `json:"Response" doc:"Status da resposta" example:"Success"`
		Message  string `json:"Message"  doc:"Mensagem de retorno" example:"Redirect successful"`
	}
}

type HealthResponse struct {
	Body struct {
		Status string `json:"status" doc:"Status do serviço" example:"ok"`
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

type DynamicResolverInput struct {
	QuickNumber string `query:"quicknumber" doc:"Número rápido do ramal PBX" example:"1001"`
	Caller      string `query:"caller"      doc:"Número do originador da chamada" example:"2000"`
	Linkedid    string `query:"linkedid"    doc:"Linked ID único da chamada" example:"1737456600.123"`
}

type PortariaWakeupInput struct {
	MoradorID   string `query:"moradorId" doc:"ID do morador"`
	ConfigID    string `query:"configId" doc:"ID da configuração de portaria autônoma"`
	Linkedid    string `query:"linkedid" doc:"Linked ID único da chamada"`
	Caller      string `query:"caller" doc:"Número originador"`
	Ramal       string `query:"ramal" doc:"Ramal WebRTC efêmero reservado"`
	SipPassword string `query:"sipPassword" doc:"Credencial efêmera do ramal"`
}

type PortariaWakeupCancelInput struct {
	SessionID string `query:"sessionId" doc:"ID da sessão de wake-up"`
	Linkedid  string `query:"linkedid" doc:"Linked ID único da chamada"`
}

type RawJSONResponse struct {
	Body json.RawMessage
}

type OpenGateInput struct {
	DeviceID string `query:"device_id" doc:"ID do dispositivo da cancela/portão" example:"1"`
	Linkedid string `query:"linkedid"  doc:"Linked ID único da chamada" example:"1737456600.123"`
}

type ResolverConfig struct {
	CondominioID     int    `json:"condominioId" doc:"Identificador do condomínio" example:"1"`
	OpenGateDigit    string `json:"openGateDigit" doc:"Dígito DTMF para abertura de cancela" example:"9"`
	OpenGateDeviceID *int   `json:"openGateDeviceId" doc:"ID do dispositivo da cancela/portão" example:"1"`
	DialTimeout      int    `json:"dial_timeout" doc:"Tempo máximo de cada tentativa de discagem, em segundos" example:"45"`
}

type ResolverContact struct {
	ID           int    `json:"id" doc:"Identificador do contato" example:"1"`
	Dial         string `json:"dial,omitempty" doc:"Número físico para discagem" example:"21995451302"`
	MoradorID    string `json:"moradorId,omitempty" doc:"Identificador do morador quando o app deve ser priorizado" example:"1"`
	Name         string `json:"name" doc:"Nome exibido" example:"João da Silva"`
	Type         string `json:"type" doc:"Tipo físico: cellphone ou portaria; app é aceito apenas por compatibilidade" example:"cellphone"`
	Repeat       int    `json:"repeat" doc:"Quantidade de repetições do ciclo app mais telefone" example:"2"`
	PriorizarApp bool   `json:"priorizarApp,omitempty" doc:"Tenta o app antes do número físico deste mesmo contato" example:"true"`
}

type ResolverResponse struct {
	Body struct {
		Config   ResolverConfig    `json:"config" doc:"Configuração retornada para o ramal"`
		Contacts []ResolverContact `json:"contacts" doc:"Lista de contatos para discagem sequencial"`
	}
}

type CondominiosSlugsResponse struct {
	Body []string
}

type CDRSearchInput struct {
	Linkedid string `query:"linkedid" doc:"Linked ID único da chamada" example:"1737456600.123"`
}

type CDRSearchResponse struct {
	Body struct {
		Rows []map[string]any `json:"rows" doc:"Linhas do CDR encontradas"`
	}
}

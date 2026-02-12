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

type HealthResponse struct {
	Body struct {
		Status   string `json:"status" doc:"Status do serviço" example:"ok"`
		Managers int    `json:"managers" doc:"Número de managers AMI conectados" example:"2"`
	}
}

type ErrorResponse struct {
	Error string `json:"error" doc:"Mensagem de erro"`
}

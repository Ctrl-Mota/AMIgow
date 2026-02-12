package config

type Config struct {
	APIKey               string      `json:"api_key"`
	BasePath             string      `json:"base_path"`
	ServerHost           string      `json:"server_host"`
	AMIServers           []AMIServer `json:"ami_servers"`
	ConfigAPIURL         string      `json:"config_api_url"`
	ConfigRefreshMinutes int         `json:"config_refresh_minutes"`
}

type AMIServer struct {
	ID       string    `json:"id"`
	Host     string    `json:"host"`
	Port     int       `json:"port"`
	Username string    `json:"username"`
	Password string    `json:"password"`
	Webhooks []Webhook `json:"webhooks"`
}

type Webhook struct {
	URL            string   `json:"url"`
	TimeoutSeconds int      `json:"timeout_seconds"`
	EventsFilter   []string `json:"events_filter"`
}

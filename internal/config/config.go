package config

var Current *Config

type Config struct {
	ID                   string      `json:"id"`
	APIKey               string      `json:"api_key"`
	BasePath             string      `json:"base_path"`
	ServerHost           string      `json:"server_host"`
	AMIServer            AMIServer   `json:"ami_server"`
	ApiConnect           ApiConnect  `json:"api_connect"`
	CDRDB                CDRDatabase `json:"cdr_db"`
	ConfigAPIURL         string      `json:"config_api_url"`
	ConfigRefreshMinutes int         `json:"config_refresh_minutes"`
}

type AMIServer struct {
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

type ApiConnect struct {
	Host                   string `json:"host"`
	PathResolver           string `json:"path_resolver"`
	PathOpenGate           string `json:"path_open_gate"`
	PathCondominiosSlugs   string `json:"path_condominios_slugs"`
	TimeoutSeconds         int    `json:"timeout_seconds"`
}

type CDRDatabase struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	Database string `json:"database"`
	Table    string `json:"table"`
}

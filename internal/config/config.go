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
	Dashboard            Dashboard   `json:"dashboard"`
}

type Dashboard struct {
	Enabled                   bool             `json:"enabled"`
	ServiceLevelTargetSeconds int              `json:"service_level_target_seconds"`
	ReconcileSeconds          int              `json:"reconcile_seconds"`
	PersistSnapshotSeconds    int              `json:"persist_snapshot_seconds"`
	RecommendedPollingMs      int              `json:"recommended_polling_ms"`
	MinPollingMs              int              `json:"min_polling_ms"`
	LiveTickMs                int              `json:"live_tick_ms"`
	SnapshotPath              string           `json:"snapshot_path"`
	EventsPath                string           `json:"events_path"`
	EventsRetentionDays       int              `json:"events_retention_days"`
	Alerts                    AlertThresholds  `json:"alerts"`
}

type AlertThresholds struct {
	WarningLongestWaitSeconds   int     `json:"warning_longest_wait_seconds"`
	CriticalLongestWaitSeconds  int     `json:"critical_longest_wait_seconds"`
	WarningWaiting              int     `json:"warning_waiting"`
	CriticalWaiting             int     `json:"critical_waiting"`
	WarningServiceLevel         float64 `json:"warning_service_level"`
	CriticalServiceLevel        float64 `json:"critical_service_level"`
	WarningAbandonRate          float64 `json:"warning_abandon_rate"`
	CriticalAbandonRate         float64 `json:"critical_abandon_rate"`
	WarningASASeconds           int     `json:"warning_asa_seconds"`
	CriticalASASeconds          int     `json:"critical_asa_seconds"`
	WarningPausedAgentsPercent  float64 `json:"warning_paused_agents_percent"`
	CriticalPausedAgentsPercent float64 `json:"critical_paused_agents_percent"`
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

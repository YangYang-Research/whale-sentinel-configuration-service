package shared

type (
	GWRequestBody struct {
		GWPayload        GWPayload `json:"payload"`
		RequestCreatedAt string    `json:"request_created_at"`
	}

	GWPayload struct {
		GWData GWData `json:"data"`
	}

	GWData struct {
		Type string `json:"type"`
		Key  string `json:"key"`
	}

	GWResponseBody struct {
		Status             string       `json:"status"`
		Message            string       `json:"message"`
		Type               string       `json:"type"`
		Key                string       `json:"key"`
		Profile            AgentProfile `json:"profile"`
		EventInfo          string       `json:"event_info"`
		RequestCreatedAt   string       `json:"request_created_at"`
		RequestProcessedAt string       `json:"request_processed_at"`
	}

	AgentProfile struct {
		RunningMode                   string                      `json:"running_mode"`
		LastRunMode                   string                      `json:"last_run_mode"`
		LiteModeDataIsSynchronized    bool                        `json:"lite_mode_data_is_synchronized"`
		LiteModeDataSynchronizeStatus string                      `json:"lite_mode_data_synchronize_status"`
		WebAttackDetection            WebAttackDetectionConfig    `json:"ws_module_web_attack_detection"`
		DGADetection                  DGADetectionConfig          `json:"ws_module_dga_detection"`
		CommonAttackDetection         CommonAttackDetectionConfig `json:"ws_module_common_attack_detection"`
		SecureResponseHeaders         SecureResponseHeaderConfig  `json:"secure_response_headers"`
	}

	WebAttackDetectionConfig struct {
		Enable       bool `json:"enable"`
		DetectHeader bool `json:"detect_header"`
		Threshold    int  `json:"threshold"`
	}

	DGADetectionConfig struct {
		Enable    bool `json:"enable"`
		Threshold int  `json:"threshold"`
	}

	CommonAttackDetectionConfig struct {
		Enable                   bool `json:"enable"`
		DetectCrossSiteScripting bool `json:"detect_cross_site_scripting"`
		DetectSqlInjection       bool `json:"detect_sql_injection"`
		DetectHTTPVerbTampering  bool `json:"detect_http_verb_tampering"`
		DetectHTTPLargeRequest   bool `json:"detect_http_large_request"`
	}

	SecureResponseHeaderConfig struct {
		Enable        bool                   `json:"enable"`
		SecureHeaders map[string]interface{} `json:"headers"`
	}

	ErrorResponse struct {
		Status    string `json:"status"`
		Message   string `json:"message"`
		ErrorCode int    `json:"error_code"`
	}
)

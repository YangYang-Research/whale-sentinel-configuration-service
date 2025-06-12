package shared

type (
	CFGP_RequestBody struct {
		EventInfo        string       `json:"event_info"`
		CFGP_Payload     CFGP_Payload `json:"payload"`
		RequestCreatedAt string       `json:"request_created_at"`
	}

	CFGP_Payload struct {
		CFGP_Data CFGP_Data `json:"data"`
	}

	CFGP_Data struct {
		Type string `json:"type"`
		Name string `json:"name"`
		Id   string `json:"id"`
	}

	CFGP_ResponseBody struct {
		Status             string            `json:"status"`
		Message            string            `json:"message"`
		Data               CFGP_ResponseData `json:"data"`
		EventInfo          string            `json:"event_info"`
		RequestCreatedAt   string            `json:"request_created_at"`
		RequestProcessedAt string            `json:"request_processed_at"`
	}

	CFGP_ResponseData struct {
		Type    string `json:"type"`
		Name    string `json:"name"`
		Id      string `json:"id"`
		Profile string `json:"profile"`
	}

	CFPS_RequestBody struct {
		EventInfo        string       `json:"event_info"`
		CFPS_Payload     CFPS_Payload `json:"payload"`
		RequestCreatedAt string       `json:"request_created_at"`
	}

	CFPS_Payload struct {
		CFPS_Data CFPS_Data `json:"data"`
	}

	CFPS_Data struct {
		Type    string                 `json:"type"`
		Name    string                 `json:"name"`
		Id      string                 `json:"id"`
		Profile map[string]interface{} `json:"profile"`
	}

	CFPS_ResponseBody struct {
		Status             string            `json:"status"`
		Message            string            `json:"message"`
		Data               CFPS_ResponseData `json:"data"`
		EventInfo          string            `json:"event_info"`
		RequestCreatedAt   string            `json:"request_created_at"`
		RequestProcessedAt string            `json:"request_processed_at"`
	}

	CFPS_ResponseData struct {
		Type    string `json:"type"`
		Name    string `json:"name"`
		Id      string `json:"id"`
		Profile string `json:"profile"`
	}

	CFMCP_RequestBody struct {
		EventInfo        string        `json:"event_info"`
		CFMCP_Payload    CFMCP_Payload `json:"payload"`
		RequestCreatedAt string        `json:"request_created_at"`
	}

	CFMCP_ResponseBody struct {
		Status             string `json:"status"`
		Message            string `json:"message"`
		RequestCreatedAt   string `json:"request_created_at"`
		RequestProcessedAt string `json:"request_processed_at"`
	}

	CFMCP_Payload struct {
		CFMCP_Data CFMCP_Data `json:"data"`
	}

	CFMCP_Data struct {
		Type string `json:"type"`
		Name string `json:"name"`
		Id   string `json:"id"`
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

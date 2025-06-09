package validation

import (
	"fmt"
	"regexp"
	"time"

	"github.com/YangYang-Research/whale-sentinel-services/whale-sentinel-configuration-service/shared"
)

// Helper functions
func ValidateCFGP_Request(req shared.CFGP_RequestBody) error {
	if req.CFGP_Payload.CFGP_Data.Type == "" || req.CFGP_Payload.CFGP_Data.Name == "" || req.CFGP_Payload.CFGP_Data.Id == "" {
		return fmt.Errorf("missing required fields")
	}

	if !regexp.MustCompile(`^[a-zA-Z0-9_-]+$`).MatchString(req.CFGP_Payload.CFGP_Data.Type) {
		return fmt.Errorf("invalid type format")
	}

	if req.CFGP_Payload.CFGP_Data.Type != "agent" && req.CFGP_Payload.CFGP_Data.Type != "service" {
		return fmt.Errorf("type must be either 'agent' or 'service'")
	}

	if _, err := time.Parse(time.RFC3339, req.RequestCreatedAt); err != nil {
		return fmt.Errorf("invalid timestamp format")
	}
	return nil
}

func ValidateCFPS_Request(req shared.CFPS_RequestBody) error {
	if req.CFPS_Payload.CFPS_Data.Type == "" || req.CFPS_Payload.CFPS_Data.Name == "" || req.CFPS_Payload.CFPS_Data.Id == "" {
		return fmt.Errorf("missing required fields")
	}

	if !regexp.MustCompile(`^[a-zA-Z0-9_-]+$`).MatchString(req.CFPS_Payload.CFPS_Data.Type) {
		return fmt.Errorf("invalid type format")
	}

	if req.CFPS_Payload.CFPS_Data.Type != "agent" && req.CFPS_Payload.CFPS_Data.Type != "service" {
		return fmt.Errorf("type must be either 'agent' or 'service'")
	}

	if _, err := time.Parse(time.RFC3339, req.RequestCreatedAt); err != nil {
		return fmt.Errorf("invalid timestamp format")
	}
	return nil
}

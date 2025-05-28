package validation

import (
	"fmt"
	"regexp"
	"time"

	"github.com/YangYang-Research/whale-sentinel-services/ws-configuration-service/shared"
)

// Helper functions
func ValidateCFRequest(req shared.CFRequestBody) error {
	if req.CFPayload.CFData.Type == "" || req.CFPayload.CFData.Key == "" {
		return fmt.Errorf("missing required fields")
	}

	if !regexp.MustCompile(`^[a-zA-Z0-9_-]+$`).MatchString(req.CFPayload.CFData.Type) {
		return fmt.Errorf("invalid type format")
	}

	if req.CFPayload.CFData.Type != "agent" && req.CFPayload.CFData.Type != "service" {
		return fmt.Errorf("type must be either 'agent' or 'service'")
	}

	if _, err := time.Parse(time.RFC3339, req.RequestCreatedAt); err != nil {
		return fmt.Errorf("invalid timestamp format")
	}
	return nil
}

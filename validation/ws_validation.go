package validation

import (
	"fmt"
	"regexp"
	"time"

	"github.com/YangYang-Research/whale-sentinel-services/ws-configuration-service/shared"
)

// Helper functions
func ValidateGWRequest(req shared.GWRequestBody) error {
	if req.GWPayload.GWData.Type == "" || req.GWPayload.GWData.Key == "" {
		return fmt.Errorf("missing required fields")
	}

	if !regexp.MustCompile(`^[a-zA-Z0-9_-]+$`).MatchString(req.GWPayload.GWData.Type) {
		return fmt.Errorf("invalid type format")
	}

	if _, err := time.Parse(time.RFC3339, req.RequestCreatedAt); err != nil {
		return fmt.Errorf("invalid timestamp format")
	}
	return nil
}

package helper

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/YangYang-Research/whale-sentinel-services/whale-sentinel-configuration-service/shared"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go/aws"
)

func GetAWSSecret(awsRegion string, secretName string, secretKeyName string) (string, error) {
	config, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(awsRegion))
	if err != nil {
		log.Fatal(err)
	}

	// Create Secrets Manager client
	svc := secretsmanager.NewFromConfig(config)

	input := &secretsmanager.GetSecretValueInput{
		SecretId:     aws.String(secretName),
		VersionStage: aws.String("AWSCURRENT"), // VersionStage defaults to AWSCURRENT if unspecified
	}

	result, err := svc.GetSecretValue(context.TODO(), input)
	if err != nil {
		// For a list of exceptions thrown, see
		// https://docs.aws.amazon.com/secretsmanager/latest/apireference/API_GetSecretValue.html
		log.Fatal(err.Error())
	}

	// Decrypts secret using the associated KMS key.
	var secretString string = *result.SecretString

	// Parse the JSON string to extract the apiKey
	var secretData map[string]string
	if err := json.Unmarshal([]byte(secretString), &secretData); err != nil {
		log.Fatalf("Failed to parse secret string: %v", err)
	}

	secretVaule, exists := secretData[secretKeyName]
	if !exists {
		log.Fatalf("apiKey not found in secret string")
	}

	// Use the apiKey as needed
	return secretVaule, nil
}

func GetDomain(fullUrl string) (string, error) {
	parsedUrl, err := url.Parse(fullUrl)
	if err != nil {
		return "", err
	}
	return parsedUrl.Host, nil
}

func GenerateCFGP_EventInfo(req shared.CFGP_RequestBody) (string, string) {
	hashInput := req.CFGP_Payload.CFGP_Data.Type + "|" + req.CFGP_Payload.CFGP_Data.Key + "|" + req.RequestCreatedAt
	eventID := sha256.Sum256([]byte(hashInput))
	eventInfo := req.CFGP_Payload.CFGP_Data.Key + "|" + "WS_CONFIGURATION_SERVICE" + "|" + hex.EncodeToString(eventID[:])
	return eventInfo, hex.EncodeToString(eventID[:])
}

func SendErrorResponse(w http.ResponseWriter, message string, errorCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(errorCode)
	json.NewEncoder(w).Encode(shared.ErrorResponse{
		Status:    "error",
		Message:   message,
		ErrorCode: errorCode,
	})
}

func ExtractEventInfo(enventInfo string) (string, string, string, error) {
	// Split the event_id by the "|" delimiter
	parts := strings.Split(enventInfo, "|")

	// Ensure the split result has exactly 3 parts
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("invalid event_info format: %s", enventInfo)
	}

	// Return the extracted components
	return parts[0], parts[1], parts[2], nil
}

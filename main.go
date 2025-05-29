package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/YangYang-Research/whale-sentinel-services/ws-configuration-service/helper"
	"github.com/YangYang-Research/whale-sentinel-services/ws-configuration-service/logger"
	"github.com/YangYang-Research/whale-sentinel-services/ws-configuration-service/shared"
	"github.com/YangYang-Research/whale-sentinel-services/ws-configuration-service/validation"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

var (
	ctx         = context.Background()
	log         *logrus.Logger
	redisClient *redis.Client
)

// Load environment variables
func init() {
	// Initialize the application logger
	log = logrus.New()
	log.SetFormatter(&logrus.TextFormatter{})
	log.SetOutput(os.Stdout)
	log.SetLevel(logrus.DebugLevel)

	if err := godotenv.Load(); err != nil {
		log.WithFields(logrus.Fields{
			"msg": err,
		}).Error("Error loading .env file")
	} else {
		log.Info("Loaded environment variables from .env file")
	}

	redisClient = redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_HOST") + ":" + os.Getenv("REDIS_PORT"),
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       0,
	})

	// Check Redis connection
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.WithFields(logrus.Fields{
			"msg": err,
		}).Error("Error connecting to Redis")
	} else {
		log.Info("Connected to Redis")
	}
}

// handlerRedis set and get value from Redis
func handlerRedis(key string, value string) (string, error) {
	if value == "" {
		// Get value from Redis
		val, err := redisClient.Get(ctx, key).Result()
		if err != nil {
			log.WithFields(logrus.Fields{
				"msg": err,
			}).Error("Error getting value from Redis")
		}
		return val, nil
	} else {
		// Set value in Redis
		err := redisClient.Set(ctx, key, value, 0).Err()
		if err != nil {
			log.WithFields(logrus.Fields{
				"msg": err,
			}).Error("Error setting value in Redis")
		}
		return key, nil
	}
}

// handleConfiguration processes incoming requests
func handleConfiguration(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		helper.SendErrorResponse(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	var req shared.CFRequestBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		helper.SendErrorResponse(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	if err := validation.ValidateCFRequest(req); err != nil {
		helper.SendErrorResponse(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Extract components from event_id
	agentID, serviceName, eventID, err := helper.ExtractEventInfo(req.EventInfo)
	if err != nil {
		helper.SendErrorResponse(w, "Error extracting event_id: %v", http.StatusBadRequest)
		return
	}

	var (
		profile       string
		getProfileErr error
		wg            sync.WaitGroup
	)

	wg.Add(1)
	go func() {
		defer wg.Done()
		profile, getProfileErr = processGetProfile(req)
	}()

	wg.Wait()
	if getProfileErr != nil {
		log.WithFields(logrus.Fields{
			"msg": getProfileErr,
		}).Error("Error processing get profile request")
	}
	eventInfo := strings.Replace(req.EventInfo, serviceName, "WS_CONFIGURATION_SERVICE", -1)
	response := shared.CFResponseBody{
		Status:             "success",
		Message:            "Profile retrieved successfully",
		Type:               req.CFPayload.CFData.Type,
		Key:                req.CFPayload.CFData.Key,
		Profile:            profile,
		EventInfo:          eventInfo,
		RequestCreatedAt:   req.RequestCreatedAt,
		RequestProcessedAt: time.Now().Format(time.RFC3339),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)

	handlerRedis(req.CFPayload.CFData.Key, profile)
	// Log the request to the logg collector
	go func(agentID string, eventInfo string, rawRequest string) {
		// Log the request to the log collector
		logData := map[string]interface{}{
			"name":                 "ws-configuration-service",
			"agent_id":             agentID,
			"source":               strings.ToLower(serviceName),
			"destination":          "ws-configuration-service",
			"event_info":           eventInfo,
			"event_id":             eventID,
			"type":                 "SERVICE_EVENT",
			"request_created_at":   req.RequestCreatedAt,
			"request_processed_at": time.Now().Format(time.RFC3339),
			"title":                "Received request from agent",
			"raw_request":          rawRequest,
			"timestamp":            time.Now().Format(time.RFC3339),
		}

		logger.Log("INFO", "ws-configuration-service", logData)
	}(agentID, eventInfo, (req.CFPayload.CFData.Type + " " + req.CFPayload.CFData.Key))
}

func processGetProfile(req shared.CFRequestBody) (string, error) {
	log.WithFields(logrus.Fields{
		"msg": "Key :" + req.CFPayload.CFData.Key,
	}).Debug("Processing get profile request")

	requestBody := map[string]interface{}{
		"event_info": req.EventInfo,
		"key":        req.CFPayload.CFData.Key,
		"type":       req.CFPayload.CFData.Type,
	}

	responseData, err := makeHTTPRequest(os.Getenv("WS_CONTROLLER_PROCESSOR_URL"), os.Getenv("WS_CONTROLLER_PROCESSOR_ENDPOINT")+"/profile", requestBody)
	if err != nil {
		return "", err
	}

	var response map[string]interface{}
	if err := json.Unmarshal(responseData, &response); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %v", err)
	}

	data := response["data"].(map[string]interface{})
	return data["profile"].(string), nil
}

func makeHTTPRequest(url, endpoint string, body interface{}) ([]byte, error) {
	jsonData, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %v", err)
	}

	apiKey, err := getAPIKey()
	if err != nil {
		return nil, fmt.Errorf("failed to get API key: %v", err)
	}

	req, err := http.NewRequest("POST", url+endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	auth := "ws:" + apiKey
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(auth)))

	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call endpoint: %v", err)
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

// getAPIKey retrieves the API key based on the configuration
func getAPIKey() (string, error) {
	awsRegion := os.Getenv("AWS_REGION")
	awsSecretName := os.Getenv("AWS_SECRET_NAME")
	awsAPISecretKeyName := os.Getenv("AWS_API_SECRET_KEY_NAME")

	awsAPIKeyVaule, err := helper.GetAWSSecret(awsRegion, awsSecretName, awsAPISecretKeyName)

	return awsAPIKeyVaule, err
}

// apiKeyAuthMiddleware is a middleware that handles API Key authentication
func apiKeyAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey, err := getAPIKey()
		if err != nil {
			helper.SendErrorResponse(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			helper.SendErrorResponse(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Decode the Base64-encoded Authorization header
		authHeader = authHeader[len("Basic "):]
		decodedAuthHeader, err := base64.StdEncoding.DecodeString(authHeader)
		if err != nil {
			helper.SendErrorResponse(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		expectedAuthValue := fmt.Sprintf("ws:%s", apiKey)
		if string(decodedAuthHeader) != expectedAuthValue {
			helper.SendErrorResponse(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Main function
func main() {
	log.Info("WS Configuration Service is running on port 5004...")
	// Initialize the logger
	logMaxSize, _ := strconv.Atoi(os.Getenv("LOG_MAX_SIZE"))
	logMaxBackups, _ := strconv.Atoi(os.Getenv("LOG_MAX_BACKUPS"))
	logMaxAge, _ := strconv.Atoi(os.Getenv("LOG_MAX_AGE"))
	logCompression, _ := strconv.ParseBool(os.Getenv("LOG_COMPRESSION"))
	logger.SetupWSLogger("ws-configuration-service", logMaxSize, logMaxBackups, logMaxAge, logCompression)
	// Wrap the handler with a 30-second timeout
	timeoutHandlerCF := http.TimeoutHandler(apiKeyAuthMiddleware(http.HandlerFunc(handleConfiguration)), 30*time.Second, "Request timed out")
	// timeoutHandlerCFS := http.TimeoutHandler(apiKeyAuthMiddleware(http.HandlerFunc(handleConfigurationSynchronize)), 30*time.Second, "Request timed out")
	// Register the timeout handler
	http.Handle("/api/v1/ws/services/configuration", timeoutHandlerCF)
	// http.Handle("/api/v1/ws/services/configuration-synchronize", timeoutHandlerCF)
	log.Fatal(http.ListenAndServe(":5004", nil))
}

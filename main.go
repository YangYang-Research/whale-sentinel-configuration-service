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

	"github.com/YangYang-Research/whale-sentinel-services/whale-sentinel-configuration-service/helper"
	"github.com/YangYang-Research/whale-sentinel-services/whale-sentinel-configuration-service/logger"
	"github.com/YangYang-Research/whale-sentinel-services/whale-sentinel-configuration-service/shared"
	"github.com/YangYang-Research/whale-sentinel-services/whale-sentinel-configuration-service/validation"
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

// handleConfigurationGetProfile processes incoming requests
func handleConfigurationGetProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		helper.SendErrorResponse(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	var req shared.CFGP_RequestBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		helper.SendErrorResponse(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	if err := validation.ValidateCFGP_Request(req); err != nil {
		helper.SendErrorResponse(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Extract components from event_id
	_, serviceName, eventID, err := helper.ExtractEventInfo(req.EventInfo)
	if err != nil {
		helper.SendErrorResponse(w, "Error extracting event_id: %v", http.StatusBadRequest)
		return
	}

	newEventInfo := strings.Replace(req.EventInfo, serviceName, "WS_CONFIGURATION_SERVICE", -1)

	var (
		status        string
		profile       string
		getProfileErr error
		wg            sync.WaitGroup
	)

	wg.Add(1)
	go func() {
		defer wg.Done()
		status, profile, getProfileErr = processGetProfile(req)
	}()

	wg.Wait()
	if getProfileErr != nil || status != "Success" || profile == "" {
		log.WithFields(logrus.Fields{
			"msg": getProfileErr,
		}).Error("Profile retrieval failed.")

		response := shared.CFGP_ResponseBody{
			Status:  status,
			Message: "Profile retrieval failed.",
			Data: shared.CFGP_ResponseData{
				Type:    req.CFGP_Payload.CFGP_Data.Type,
				Name:    req.CFGP_Payload.CFGP_Data.Name,
				Id:      req.CFGP_Payload.CFGP_Data.Id,
				Profile: "",
			},
			EventInfo:          newEventInfo,
			RequestCreatedAt:   req.RequestCreatedAt,
			RequestProcessedAt: time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)

		log.Infof("POST %v - 200", r.URL)
		// Log the request to the logg collector
		go func(Id string, Name string, Type string, eventInfo string, rawRequest interface{}) {
			// Determine action_type based on Type
			actionType := "SERVICE_GET_AGENT_PROFILE"
			actionResult := "SERVICE_GET_AGENT_PROFILE_FAILED"
			nameOfAgent := Name
			nameOfService := ""
			if Type == "service" {
				actionType = "SERVICE_GET_SERVICE_PROFILE"
				actionResult = "SERVICE_GET_SERVICE_PROFILE_FAILED"
				nameOfService = Name
				nameOfAgent = ""
			}
			// Log the request to the log collector
			logData := map[string]interface{}{
				"service":              "ws-configuration-service",
				"agent_id":             Id,
				"agent_name":           nameOfAgent,
				"service_name":         nameOfService,
				"source":               strings.ToLower(serviceName),
				"destination":          "ws-configuration-service",
				"event_info":           eventInfo,
				"event_id":             eventID,
				"type":                 "SERVICE_TO_SERVICE_EVENT",
				"action_type":          actionType,
				"action_result":        actionResult,
				"action_status":        "FAILED",
				"request_created_at":   req.RequestCreatedAt,
				"request_processed_at": time.Now().UTC().Format("2006-01-02T15:04:05Z"),
				"message":              "Cannot fetch agent profile from ws-controller-processor.",
				"raw_request":          rawRequest,
			}

			logger.Log("INFO", logData)
		}(req.CFGP_Payload.CFGP_Data.Id, req.CFGP_Payload.CFGP_Data.Name, req.CFGP_Payload.CFGP_Data.Type, newEventInfo, (req))
		return
	}

	response := shared.CFGP_ResponseBody{
		Status:  status,
		Message: "Profile retrieved successfully.",
		Data: shared.CFGP_ResponseData{
			Type:    req.CFGP_Payload.CFGP_Data.Type,
			Name:    req.CFGP_Payload.CFGP_Data.Name,
			Id:      req.CFGP_Payload.CFGP_Data.Id,
			Profile: profile,
		},
		EventInfo:          newEventInfo,
		RequestCreatedAt:   req.RequestCreatedAt,
		RequestProcessedAt: time.Now().UTC().Format("2006-01-02T15:04:05Z"),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)

	if profile != "" {
		handlerRedis(req.CFGP_Payload.CFGP_Data.Name, profile)
	}
	log.Infof("POST %v - 200", r.URL)
	// Log the request to the logg collector
	go func(Id string, Name string, Type string, eventInfo string, rawRequest interface{}) {
		// Determine action_type based on Type
		actionType := "SERVICE_GET_AGENT_PROFILE"
		actionResult := "SERVICE_GET_AGENT_PROFILE_SUCCESSED"
		nameOfAgent := Name
		nameOfService := ""
		if Type == "service" {
			actionType = "SERVICE_GET_SERVICE_PROFILE"
			actionResult = "SERVICE_GET_SERVICE_PROFILE_SUCCESSED"
			nameOfService = Name
			nameOfAgent = ""

		}
		// Log the request to the log collector
		logData := map[string]interface{}{
			"service":              "ws-configuration-service",
			"agent_id":             Id,
			"agent_name":           nameOfAgent,
			"service_name":         nameOfService,
			"source":               strings.ToLower(serviceName),
			"destination":          "ws-configuration-service",
			"event_info":           eventInfo,
			"event_id":             eventID,
			"type":                 "SERVICE_TO_SERVICE_EVENT",
			"action_type":          actionType,
			"action_result":        actionResult,
			"action_status":        "SUCCESSED",
			"request_created_at":   req.RequestCreatedAt,
			"request_processed_at": time.Now().UTC().Format("2006-01-02T15:04:05Z"),
			"message":              "Profile retrieved successfully.",
			"raw_request":          rawRequest,
			"timestamp":            time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		}

		logger.Log("INFO", logData)
	}(req.CFGP_Payload.CFGP_Data.Id, req.CFGP_Payload.CFGP_Data.Name, req.CFGP_Payload.CFGP_Data.Type, newEventInfo, (req))
}

func processGetProfile(req shared.CFGP_RequestBody) (string, string, error) {
	log.WithFields(logrus.Fields{
		"msg": "Name :" + req.CFGP_Payload.CFGP_Data.Name,
	}).Debug("Processing get profile request")

	requestBody := map[string]interface{}{
		"event_info": req.EventInfo,
		"payload": map[string]interface{}{
			"data": map[string]interface{}{
				"name": req.CFGP_Payload.CFGP_Data.Name,
				"id":   req.CFGP_Payload.CFGP_Data.Id,
				"type": req.CFGP_Payload.CFGP_Data.Type,
			},
		},
		"request_created_at": req.RequestCreatedAt,
	}

	responseData, err := makeHTTPRequest(os.Getenv("WS_CONTROLLER_PROCESSOR_URL"), os.Getenv("WS_CONTROLLER_PROCESSOR_ENDPOINT")+"/profile", requestBody)
	if err != nil {
		return "Error", "", err
	}

	var response map[string]interface{}
	if err := json.Unmarshal(responseData, &response); err != nil {
		return response["status"].(string), "", fmt.Errorf("failed to unmarshal response: %v", err)
	}

	if response["status"] != "Success" {
		return response["status"].(string), "", fmt.Errorf("failed to get profile: %s", response["message"])
	}

	data := response["data"].(map[string]interface{})
	return response["status"].(string), data["profile"].(string), nil
}

// handleConfigurationProfileSynchronize processes incoming requests
func handleConfigurationProfileSynchronize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		helper.SendErrorResponse(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	var req shared.CFPS_RequestBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		helper.SendErrorResponse(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	if err := validation.ValidateCFPS_Request(req); err != nil {
		helper.SendErrorResponse(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Extract components from event_id
	_, serviceName, eventID, err := helper.ExtractEventInfo(req.EventInfo)
	if err != nil {
		helper.SendErrorResponse(w, "Error extracting event_id: %v", http.StatusBadRequest)
		return
	}
	newEventInfo := strings.Replace(req.EventInfo, serviceName, "WS_CONFIGURATION_SERVICE", -1)

	var (
		status        string
		profile       string
		getProfileErr error
		wg            sync.WaitGroup
	)

	wg.Add(1)
	go func() {
		defer wg.Done()
		status, profile, getProfileErr = processProfileSynchronize(req)
	}()

	wg.Wait()
	if getProfileErr != nil || status != "Success" || profile == "" {
		log.WithFields(logrus.Fields{
			"msg": getProfileErr,
		}).Error("Profile synchronization failed.")

		response := shared.CFPS_ResponseBody{
			Status:  status,
			Message: "Profile synchronization failed.",
			Data: shared.CFPS_ResponseData{
				Type:    req.CFPS_Payload.CFPS_Data.Type,
				Name:    req.CFPS_Payload.CFPS_Data.Name,
				Id:      req.CFPS_Payload.CFPS_Data.Id,
				Profile: "",
			},
			EventInfo:          newEventInfo,
			RequestCreatedAt:   req.RequestCreatedAt,
			RequestProcessedAt: time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)

		log.Infof("POST %v - 200", r.URL)
		// Log the request to the logg collector
		go func(Id string, Name string, Type string, eventInfo string, rawRequest interface{}) {
			// Determine action_type based on Type
			actionType := "SERVICE_SYNC_AGENT_PROFILE"
			actionResult := "SERVICE_SYNC_AGENT_PROFILE_FAILED"
			nameOfAgent := Name
			nameOfService := ""
			if Type == "service" {
				actionType = "SERVICE_SYNC_SERVICE_PROFILE"
				actionResult = "SERVICE_SYNC_SERVICE_PROFILE_FAILED"
				nameOfService = Name
				nameOfAgent = ""
			}
			// Log the request to the log collector
			logData := map[string]interface{}{
				"service":              "ws-configuration-service",
				"agent_id":             Id,
				"agent_name":           nameOfAgent,
				"service_name":         nameOfService,
				"source":               strings.ToLower(serviceName),
				"destination":          "ws-configuration-service",
				"event_info":           eventInfo,
				"event_id":             eventID,
				"type":                 "SERVICE_TO_SERVICE_EVENT",
				"action_type":          actionType,
				"action_result":        actionResult,
				"action_status":        "FAILED",
				"request_created_at":   req.RequestCreatedAt,
				"request_processed_at": time.Now().UTC().Format("2006-01-02T15:04:05Z"),
				"message":              "Cannot synchronize agent profile with ws-controller-processor.",
				"raw_request":          rawRequest,
			}

			logger.Log("INFO", logData)
		}(req.CFPS_Payload.CFPS_Data.Id, req.CFPS_Payload.CFPS_Data.Name, req.CFPS_Payload.CFPS_Data.Type, newEventInfo, (req))
		return
	}

	response := shared.CFPS_ResponseBody{
		Status:  status,
		Message: "Profile synchronized successfully.",
		Data: shared.CFPS_ResponseData{
			Type:    req.CFPS_Payload.CFPS_Data.Type,
			Name:    req.CFPS_Payload.CFPS_Data.Name,
			Id:      req.CFPS_Payload.CFPS_Data.Id,
			Profile: profile,
		},
		EventInfo:          newEventInfo,
		RequestCreatedAt:   req.RequestCreatedAt,
		RequestProcessedAt: time.Now().UTC().Format("2006-01-02T15:04:05Z"),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)

	if status == "Success" && profile != "" {
		handlerRedis(req.CFPS_Payload.CFPS_Data.Name, profile)
	}
	log.Infof("POST %v - 200", r.URL)
	// Log the request to the logg collector
	go func(Id string, Name string, Type string, eventInfo string, rawRequest interface{}) {
		// Determine action_type based on Type
		actionType := "SERVICE_SYNC_AGENT_PROFILE"
		actionResult := "SERVICE_SYNC_AGENT_PROFILE_SUCCESSED"
		nameOfAgent := Name
		nameOfService := ""
		if Type == "service" {
			actionType = "SERVICE_SYNC_SERVICE_PROFILE"
			actionResult = "SERVICE_SYNC_SERVICE_PROFILE_SUCCESSED"
			nameOfService = Name
			nameOfAgent = ""
		}
		// Log the request to the log collector
		logData := map[string]interface{}{
			"service":              "ws-configuration-service",
			"agent_id":             Id,
			"agent_name":           nameOfAgent,
			"service_name":         nameOfService,
			"source":               strings.ToLower(serviceName),
			"destination":          "ws-configuration-service",
			"event_info":           eventInfo,
			"event_id":             eventID,
			"type":                 "SERVICE_TO_SERVICE_EVENT",
			"action_type":          actionType,
			"action_result":        actionResult,
			"action_status":        "SUCCESSED",
			"request_created_at":   req.RequestCreatedAt,
			"request_processed_at": time.Now().UTC().Format("2006-01-02T15:04:05Z"),
			"message":              "Profile synchronized successfully.",
			"raw_request":          rawRequest,
			"timestamp":            time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		}

		logger.Log("INFO", logData)
	}(req.CFPS_Payload.CFPS_Data.Id, req.CFPS_Payload.CFPS_Data.Name, req.CFPS_Payload.CFPS_Data.Type, newEventInfo, (req))
}

func processProfileSynchronize(req shared.CFPS_RequestBody) (string, string, error) {
	log.WithFields(logrus.Fields{
		"msg": "Key :" + req.CFPS_Payload.CFPS_Data.Name,
	}).Debug("Processing get profile request")

	requestBody := map[string]interface{}{
		"event_info": req.EventInfo,
		"payload": map[string]interface{}{
			"data": map[string]interface{}{
				"name":             req.CFPS_Payload.CFPS_Data.Name,
				"id":               req.CFPS_Payload.CFPS_Data.Id,
				"type":             req.CFPS_Payload.CFPS_Data.Type,
				"profile":          req.CFPS_Payload.CFPS_Data.Profile,
				"host_information": req.CFPS_Payload.CFPS_Data.HostInformation,
			},
		},
		"request_created_at": req.RequestCreatedAt,
	}

	responseData, err := makeHTTPRequest(os.Getenv("WS_CONTROLLER_PROCESSOR_URL"), os.Getenv("WS_CONTROLLER_PROCESSOR_ENDPOINT")+"/profile/synchronize", requestBody)
	if err != nil {
		return "Error", "", err
	}

	var response map[string]interface{}
	if err := json.Unmarshal(responseData, &response); err != nil {
		return response["status"].(string), "", fmt.Errorf("failed to unmarshal response: %v", err)
	}

	if response["status"] != "Success" {
		return response["status"].(string), "", fmt.Errorf("failed to sync profile: %s", response["message"])
	}

	data := response["data"].(map[string]interface{})
	return response["status"].(string), data["profile"].(string), nil
}

func handleConfigurationMCPSynchronize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		helper.SendErrorResponse(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	var req shared.CFMCP_RequestBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		helper.SendErrorResponse(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	if err := validation.ValidateCFMCP_Request(req); err != nil {
		helper.SendErrorResponse(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Extract components from event_id
	_, serviceName, eventID, err := helper.ExtractEventInfo(req.EventInfo)
	if err != nil {
		helper.SendErrorResponse(w, "Error extracting event_id: %v", http.StatusBadRequest)
		return
	}

	newEventInfo := strings.Replace(req.EventInfo, serviceName, "WS_CONFIGURATION_SERVICE", -1)

	var (
		status        string
		profile       string
		getProfileErr error
		wg            sync.WaitGroup
	)

	wg.Add(1)
	go func() {
		defer wg.Done()
		status, profile, getProfileErr = processMCPSyncProfile(req)
	}()

	wg.Wait()
	if getProfileErr != nil || status != "Success" || profile == "" {
		log.WithFields(logrus.Fields{
			"msg": getProfileErr,
		}).Error("Latest profile synchronization failed.")

		response := shared.CFGP_ResponseBody{
			Status:  status,
			Message: "Latest Profile synchronization failed.",
			Data: shared.CFGP_ResponseData{
				Type: req.CFMCP_Payload.CFMCP_Data.Type,
				Name: req.CFMCP_Payload.CFMCP_Data.Name,
				Id:   req.CFMCP_Payload.CFMCP_Data.Id,
			},
			EventInfo:          newEventInfo,
			RequestCreatedAt:   req.RequestCreatedAt,
			RequestProcessedAt: time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)

		log.Infof("POST %v - 200", r.URL)
		// Log the request to the logg collector
		go func(Id string, Name string, Type string, eventInfo string, rawRequest interface{}) {
			// Determine action_type based on Type
			actionType := "SERVICE_SYNC_LATEST_AGENT_PROFILE"
			actionResult := "SERVICE_SYNC_LATEST_AGENT_PROFILE_FAILED"
			nameOfService := ""
			nameOfAgent := Name
			if Type == "service" {
				actionType = "SERVICE_SYNC_LATEST_SERVICE_PROFILE"
				actionResult = "SERVICE_SYNC_LATEST_SERVICE_PROFILE_FAILED"
				nameOfService = Name
				nameOfAgent = ""
			}
			// Log the request to the log collector
			logData := map[string]interface{}{
				"service":              "ws-configuration-service",
				"agent_id":             Id,
				"agent_name":           nameOfAgent,
				"service_name":         nameOfService,
				"source":               strings.ToLower(serviceName),
				"destination":          "ws-configuration-service",
				"event_info":           eventInfo,
				"event_id":             eventID,
				"type":                 "SERVICE_TO_SERVICE_EVENT",
				"action_type":          actionType,
				"action_result":        actionResult,
				"action_status":        "FAILED",
				"request_created_at":   req.RequestCreatedAt,
				"request_processed_at": time.Now().UTC().Format("2006-01-02T15:04:05Z"),
				"message":              "Cannnot fetch latest agent profile from ws-controller-processor.",
				"raw_request":          rawRequest,
				"timestamp":            time.Now().UTC().Format("2006-01-02T15:04:05Z"),
			}

			logger.Log("INFO", logData)
		}(req.CFMCP_Payload.CFMCP_Data.Id, req.CFMCP_Payload.CFMCP_Data.Name, req.CFMCP_Payload.CFMCP_Data.Type, newEventInfo, (req))
		return
	}

	response := shared.CFGP_ResponseBody{
		Status:  status,
		Message: "Profile synchronized successfully.",
		Data: shared.CFGP_ResponseData{
			Type:    req.CFMCP_Payload.CFMCP_Data.Type,
			Name:    req.CFMCP_Payload.CFMCP_Data.Name,
			Id:      req.CFMCP_Payload.CFMCP_Data.Id,
			Profile: profile,
		},
		EventInfo:          newEventInfo,
		RequestCreatedAt:   req.RequestCreatedAt,
		RequestProcessedAt: time.Now().UTC().Format("2006-01-02T15:04:05Z"),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)

	if profile != "" {
		handlerRedis(req.CFMCP_Payload.CFMCP_Data.Name, profile)
	}

	log.Infof("POST %v - 200", r.URL)
	// Log the request to the logg collector
	go func(Id string, Name string, Type string, eventInfo string, rawRequest interface{}) {
		// Determine action_type based on Type
		actionType := "SERVICE_SYNC_LATEST_AGENT_PROFILE"
		actionResult := "SERVICE_SYNC_LATEST_AGENT_PROFILE_SUCCESSED"
		nameOfService := ""
		nameOfAgent := Name
		if Type == "service" {
			actionType = "SERVICE_SYNC_LATEST_SERVICE_PROFILE"
			actionResult = "SERVICE_SYNC_LATEST_SERVICE_PROFILE_SUCCESSED"
			nameOfService = Name
			nameOfAgent = ""
		}
		// Log the request to the log collector
		logData := map[string]interface{}{
			"service":              "ws-configuration-service",
			"agent_id":             Id,
			"agent_name":           nameOfAgent,
			"service_name":         nameOfService,
			"source":               strings.ToLower(serviceName),
			"destination":          "ws-configuration-service",
			"event_info":           eventInfo,
			"event_id":             eventID,
			"type":                 "SERVICE_TO_SERVICE_EVENT",
			"action_type":          actionType,
			"action_result":        actionResult,
			"action_status":        "SUCCESSED",
			"request_created_at":   req.RequestCreatedAt,
			"request_processed_at": time.Now().UTC().Format("2006-01-02T15:04:05Z"),
			"message":              "Latest profile synchronized successfully.",
			"raw_request":          rawRequest,
			"timestamp":            time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		}

		logger.Log("INFO", logData)
	}(req.CFMCP_Payload.CFMCP_Data.Id, req.CFMCP_Payload.CFMCP_Data.Name, req.CFMCP_Payload.CFMCP_Data.Type, newEventInfo, (req))
}

func processMCPSyncProfile(req shared.CFMCP_RequestBody) (string, string, error) {
	log.WithFields(logrus.Fields{
		"msg": "Name :" + req.CFMCP_Payload.CFMCP_Data.Name,
	}).Debug("Processing get profile request")

	requestBody := map[string]interface{}{
		"event_info": req.EventInfo,
		"payload": map[string]interface{}{
			"data": map[string]interface{}{
				"name": req.CFMCP_Payload.CFMCP_Data.Name,
				"id":   req.CFMCP_Payload.CFMCP_Data.Id,
				"type": req.CFMCP_Payload.CFMCP_Data.Type,
			},
		},
		"request_created_at": req.RequestCreatedAt,
	}

	responseData, err := makeHTTPRequest(os.Getenv("WS_CONTROLLER_PROCESSOR_URL"), os.Getenv("WS_CONTROLLER_PROCESSOR_ENDPOINT")+"/profile", requestBody)
	if err != nil {
		return "Error", "", err
	}

	var response map[string]interface{}
	if err := json.Unmarshal(responseData, &response); err != nil {
		return response["status"].(string), "", fmt.Errorf("failed to unmarshal response: %v", err)
	}

	if response["status"] != "Success" {
		return response["status"].(string), "", fmt.Errorf("failed to get profile: %s", response["message"])
	}

	data := response["data"].(map[string]interface{})
	return response["status"].(string), data["profile"].(string), nil
}

func makeHTTPRequest(url, endpoint string, body interface{}) ([]byte, error) {
	jsonData, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %v", err)
	}

	secretValue, err := getSecret(os.Getenv("WHALE_SENTINEL_CONTROLLER_SECRET_KEY_NAME"))
	if err != nil {
		return nil, fmt.Errorf("failed to get API key: %v", err)
	}

	req, err := http.NewRequest("POST", url+endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	auth := "ws:" + secretValue
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(auth)))

	verifyTLS, err := strconv.ParseBool(os.Getenv("WHALE_SENTINEL_VERIFY_TLS"))
	if err != nil {
		log.Fatalf("Invalid boolean value for WHALE_SENTINEL_VERIFY_TLS: %v", err)
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: !verifyTLS},
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call endpoint: %v", err)
	}
	defer resp.Body.Close()

	log.Infof("POST %v - %v", url+endpoint, resp.StatusCode)
	return io.ReadAll(resp.Body)
}

// getAPIKey retrieves the API key based on the configuration
func getSecret(key string) (string, error) {
	awsRegion := os.Getenv("AWS_REGION")
	awsSecretName := os.Getenv("AWS_SECRET_NAME")
	awsSecretKeyName := key

	awsSecretVaule, err := helper.GetAWSSecret(awsRegion, awsSecretName, awsSecretKeyName)

	return awsSecretVaule, err
}

// apiKeyAuthMiddleware is a middleware that handles API Key authentication
func apiKeyAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secretValue, err := getSecret(os.Getenv("WHALE_SENTINEL_SERVICE_SECRET_KEY_NAME"))
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

		expectedAuthValue := fmt.Sprintf("ws:%s", secretValue)
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
	timeoutHandlerCFGP := http.TimeoutHandler(apiKeyAuthMiddleware(http.HandlerFunc(handleConfigurationGetProfile)), 30*time.Second, "Request timed out")
	timeoutHandlerCFPS := http.TimeoutHandler(apiKeyAuthMiddleware(http.HandlerFunc(handleConfigurationProfileSynchronize)), 30*time.Second, "Request timed out")
	timeoutHandlerCFMCP := http.TimeoutHandler((http.HandlerFunc(handleConfigurationMCPSynchronize)), 30*time.Second, "Request timed out")

	// Register the timeout handler
	http.Handle("/api/v1/ws/services/configuration/profile", timeoutHandlerCFGP)
	http.Handle("/api/v1/ws/services/configuration/profile/synchronize", timeoutHandlerCFPS)
	http.Handle("/api/v1/ws/services/configuration/mcp/synchronize", timeoutHandlerCFMCP)
	log.Fatal(http.ListenAndServe(":5004", nil))
}

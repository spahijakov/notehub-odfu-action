package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sethvargo/go-githubactions"
)

func main() {
	ctx := context.Background()

	// Initialize GitHub Actions
	action := githubactions.New()

	// Get required inputs
	projectUID := action.GetInput("project_uid")
	firmwareFile := action.GetInput("firmware_file")

	// Get secrets
	clientID := action.GetInput("client_id")
	clientSecret := action.GetInput("client_secret")

	issueDFU := action.GetInput("issue_dfu");

	// Validate required inputs
	if projectUID == "" {
		action.Fatalf("project_uid is required")
	}
	if firmwareFile == "" {
		action.Fatalf("firmware_file is required")
	}
	if clientID == "" {
		action.Fatalf("client_id is required")
	}
	if clientSecret == "" {
		action.Fatalf("client_secret is required")
	}
	if issueDFU == "" {
		action.Fatalf("issue_dfu is required")
	}

	// Get optional inputs
	deviceUID := action.GetInput("device_uid")
	tag := action.GetInput("tag")
	serialNumber := action.GetInput("serial_number")
	fleetUID := action.GetInput("fleet_uid")
	productUID := action.GetInput("product_uid")
	notecardFirmware := action.GetInput("notecard_firmware")
	location := action.GetInput("location")
	sku := action.GetInput("sku")

	onlyUpload := strings.EqualFold(issueDFU, "false")

	log.Printf("Starting firmware deployment to Notehub...")
	log.Printf("Project UID: %s", projectUID)
	log.Printf("Firmware File: %s", firmwareFile)

	// Execute deployment
	if err := deployFirmware(ctx, &DeploymentConfig{
		ProjectUID:       projectUID,
		FirmwareFile:     firmwareFile,
		ClientID:         clientID,
		ClientSecret:     clientSecret,
		DeviceUID:        deviceUID,
		Tag:              tag,
		SerialNumber:     serialNumber,
		FleetUID:         fleetUID,
		ProductUID:       productUID,
		NotecardFirmware: notecardFirmware,
		Location:         location,
		SKU:              sku,
	}, onlyUpload); err != nil {
		action.Fatalf("Deployment failed: %v", err)
	}

	log.Printf("✅ Firmware deployment completed successfully")

}

// DeploymentConfig contains all the configuration for firmware deployment
type DeploymentConfig struct {
	ProjectUID       string
	FirmwareFile     string
	ClientID         string
	ClientSecret     string
	DeviceUID        string
	Tag              string
	SerialNumber     string
	FleetUID         string
	ProductUID       string
	NotecardFirmware string
	Location         string
	SKU              string
}

// NotehubClient handles API communication with Notehub
type NotehubClient struct {
	httpClient  *http.Client
	accessToken string
	baseURL     string
}

// OAuth2TokenResponse represents the response from OAuth2 token endpoint
type OAuth2TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

// FirmwareUploadResponse represents the response from firmware upload
type FirmwareUploadResponse struct {
	Filename string `json:"filename"`
}

// DFURequest represents the payload for triggering device firmware update
type DFURequest struct {
	Filename string `json:"filename"`
}

// DFUResponse represents the response from DFU trigger
type DFUResponse struct {
	Success bool   `json:"success,omitempty"`
	Message string `json:"message,omitempty"`
}

// NewNotehubClient creates a new Notehub API client
func NewNotehubClient() *NotehubClient {
	return &NotehubClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: "https://api.notefile.net/v1",
	}
}

// Authenticate obtains an OAuth2 access token from Notehub
func (c *NotehubClient) Authenticate(ctx context.Context, clientID, clientSecret string) error {
	log.Printf("Obtaining OAuth2 bearer token from Notehub...")

	// Prepare form data
	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", "https://notehub.io/oauth2/token", strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create OAuth2 request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("OAuth2 request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read OAuth2 response: %w", err)
	}

	// Check status code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("OAuth2 request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var tokenResp OAuth2TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return fmt.Errorf("failed to parse OAuth2 response: %w", err)
	}

	if tokenResp.AccessToken == "" {
		return fmt.Errorf("OAuth2 response missing access token")
	}

	c.accessToken = tokenResp.AccessToken
	log.Printf("✅ OAuth2 token obtained successfully")

	return nil
}

// UploadFirmware uploads a firmware binary file to Notehub
func (c *NotehubClient) UploadFirmware(ctx context.Context, projectUID, firmwareFile string) (*FirmwareUploadResponse, error) {
	log.Printf("Uploading firmware to Notehub...")

	// Read firmware file
	fileData, err := os.ReadFile(firmwareFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read firmware file: %w", err)
	}

	filename := filepath.Base(firmwareFile)
	fileSize := len(fileData)

	log.Printf("  - Project: %s", projectUID)
	log.Printf("  - File: %s", filename)
	log.Printf("  - Size: %d bytes", fileSize)

	// Create upload URL
	uploadURL := fmt.Sprintf("%s/projects/%s/firmware/host/%s", c.baseURL, projectUID, filename)

	// Create request with binary data
	req, err := http.NewRequestWithContext(ctx, "PUT", uploadURL, bytes.NewReader(fileData))
	if err != nil {
		return nil, fmt.Errorf("failed to create upload request: %w", err)
	}

	// Set headers
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("Content-Type", "application/octet-stream")

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("firmware upload request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read upload response: %w", err)
	}

	// Check status code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("firmware upload failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var uploadResp FirmwareUploadResponse
	if err := json.Unmarshal(body, &uploadResp); err != nil {
		return nil, fmt.Errorf("failed to parse upload response: %w", err)
	}

	log.Printf("✅ Firmware upload successful")
	log.Printf("✅ Captured uploaded filename: %s", uploadResp.Filename)

	return &uploadResp, nil
}

// addCommaSeparatedParams adds comma-separated values as multiple query parameters
func addCommaSeparatedParams(queryParams url.Values, paramName, value string) {
	if value == "" {
		return
	}

	if strings.Contains(value, ",") {
		values := strings.Split(value, ",")
		for _, v := range values {
			v = strings.TrimSpace(v)
			if v != "" {
				queryParams.Add(paramName, v)
			}
		}
	} else {
		queryParams.Set(paramName, value)
	}
}

// TriggerDFU initiates a device firmware update for targeted devices
func (c *NotehubClient) TriggerDFU(ctx context.Context, config *DeploymentConfig, filename string) error {
	log.Printf("Triggering device firmware update...")

	// Build query parameters from optional targeting inputs
	queryParams := url.Values{}

	addCommaSeparatedParams(queryParams, "deviceUID", config.DeviceUID)
	addCommaSeparatedParams(queryParams, "tags", config.Tag)
	addCommaSeparatedParams(queryParams, "serialNumber", config.SerialNumber)
	addCommaSeparatedParams(queryParams, "fleetUID", config.FleetUID)
	addCommaSeparatedParams(queryParams, "productUID", config.ProductUID)
	addCommaSeparatedParams(queryParams, "notecardFirmware", config.NotecardFirmware)
	addCommaSeparatedParams(queryParams, "location", config.Location)
	addCommaSeparatedParams(queryParams, "sku", config.SKU)

	// Build DFU URL
	dfuURL := fmt.Sprintf("%s/projects/%s/dfu/host/update", c.baseURL, config.ProjectUID)
	if len(queryParams) > 0 {
		dfuURL += "?" + queryParams.Encode()
	}

	log.Printf("DFU URL: %s", dfuURL)

	// Create JSON payload
	payload := DFURequest{
		Filename: filename,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal DFU payload: %w", err)
	}

	log.Printf("Payload: %s", string(payloadBytes))

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", dfuURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create DFU request: %w", err)
	}

	// Set headers
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("Content-Type", "application/json")

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("DFU request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read DFU response: %w", err)
	}

	// Check status code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("device firmware update failed with status %d: %s", resp.StatusCode, string(body))
	}

	log.Printf("✅ Device firmware update triggered successfully")
	log.Printf("Response: %s", string(body))

	return nil
}

// deployFirmware orchestrates the entire firmware deployment process
func deployFirmware(ctx context.Context, config *DeploymentConfig, onlyUpload bool) error {
	// Initialize Notehub client
	client := NewNotehubClient()

	// Step 1: Authenticate with Notehub
	if err := client.Authenticate(ctx, config.ClientID, config.ClientSecret); err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	// Step 2: Validate firmware file exists
	firmwareFile := filepath.Join("./firmware", config.FirmwareFile)
	if _, err := os.Stat(firmwareFile); os.IsNotExist(err) {
		return fmt.Errorf("firmware file not found: %s", firmwareFile)
	}

	log.Printf("✅ Input validation passed")

	// Step 3: Upload firmware to Notehub
	uploadResp, err := client.UploadFirmware(ctx, config.ProjectUID, firmwareFile)
	if err != nil {
		return fmt.Errorf("firmware upload failed: %w", err)
	}

	log.Printf("✅ Firmware uploaded to Notehub")

	if (!onlyUpload){
		// Step 4: Trigger Device Firmware Update
		if err := client.TriggerDFU(ctx, config, uploadResp.Filename); err != nil {
			return fmt.Errorf("DFU trigger failed: %w", err)
		}

		log.Printf("✅ Device firmware update triggered")

		// Step 5: Deployment Summary
		logDeploymentSummary(config, uploadResp.Filename)
	}

	return nil
}

// logDeploymentSummary prints a comprehensive deployment summary
func logDeploymentSummary(config *DeploymentConfig, filename string) {
	log.Printf("=== Deployment Summary ===")
	log.Printf("Project UID: %s", config.ProjectUID)
	log.Printf("Firmware File: %s", config.FirmwareFile)
	log.Printf("Uploaded Filename: %s", filename)

	// Log targeting parameters if specified
	if config.DeviceUID != "" {
		log.Printf("Target Device UID: %s", config.DeviceUID)
	}
	if config.Tag != "" {
		log.Printf("Target Tag: %s", config.Tag)
	}
	if config.SerialNumber != "" {
		log.Printf("Target Serial: %s", config.SerialNumber)
	}
	if config.FleetUID != "" {
		log.Printf("Fleet UID: %s", config.FleetUID)
	}
	if config.ProductUID != "" {
		log.Printf("Product UID: %s", config.ProductUID)
	}
	if config.NotecardFirmware != "" {
		log.Printf("Notecard Firmware: %s", config.NotecardFirmware)
	}
	if config.Location != "" {
		log.Printf("Location: %s", config.Location)
	}
	if config.SKU != "" {
		log.Printf("SKU: %s", config.SKU)
	}

	log.Printf("Deployment Status: SUCCESS")
}

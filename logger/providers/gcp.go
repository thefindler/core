package providers

import (
	"context"
	"fmt"

	"github.com/thefindler/shared/config"
	"cloud.google.com/go/logging"
)

// GCPProvider implements logging to Google Cloud Platform
type GCPProvider struct {
	logger *logging.Logger
	client *logging.Client
}

// NewGCPProvider creates a new GCP logging provider
// Config from LOG_CONFIG["gcp"]: {"project_id": "xxx", "project_name": "xxx", "env": "xxx"}
func NewGCPProvider() (*GCPProvider, error) {
	ctx := context.Background()
	
	projectID := GetConfigString("gcp", "project_id")
	if projectID == "" {
		return nil, fmt.Errorf("gcp.project_id not found in LOG_CONFIG")
	}

	client, err := logging.NewClient(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCP logging client: %w", err)
	}

	// Get logger name with fallback to PROJECT and ENV env vars (backward compatibility)
	projectName := GetConfigString("gcp", "project_name")
	if projectName == "" && config.IsGlobalConfigInitialized() {
		projectName = config.GetConfig("PROJECT")
	}
	
	env := GetConfigString("gcp", "env")
	if env == "" && config.IsGlobalConfigInitialized() {
		env = config.GetConfig("ENV")
	}
	
	loggerName := "default-app-logger"
	if projectName != "" && env != "" {
		loggerName = projectName + "-" + env
	}

	return &GCPProvider{
		logger: client.Logger(loggerName),
		client: client,
	}, nil
}

// Write writes a log entry to GCP Cloud Logging
func (g *GCPProvider) Write(severity, message string, payload map[string]interface{}) error {
	// Convert severity string to logging.Severity
	gcpSeverity := parseGCPSeverity(severity)
	
	entry := logging.Entry{
		Payload:  payload,
		Severity: gcpSeverity,
	}
	
	g.logger.Log(entry)
	return nil
}

// Close closes the GCP logging client
func (g *GCPProvider) Close() error {
	if g.client != nil {
		return g.client.Close()
	}
	return nil
}

// Name returns the provider name
func (g *GCPProvider) Name() string {
	return "gcp"
}

// parseGCPSeverity converts severity string to GCP logging.Severity
func parseGCPSeverity(severity string) logging.Severity {
	switch severity {
	case "DEBUG":
		return logging.Debug
	case "INFO":
		return logging.Info
	case "WARNING":
		return logging.Warning
	case "ERROR":
		return logging.Error
	case "CRITICAL":
		return logging.Critical
	default:
		return logging.Info
	}
}


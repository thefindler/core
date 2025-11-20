package providers

import (
	"fmt"
	"time"

	"github.com/microsoft/ApplicationInsights-Go/appinsights"
	"github.com/microsoft/ApplicationInsights-Go/appinsights/contracts"
)

// AzureProvider implements logging to Azure Application Insights
type AzureProvider struct {
	client appinsights.TelemetryClient
}

// NewAzureProvider creates a new Azure Application Insights logging provider
// Config from LOG_CONFIG["azure"]: {"instrumentation_key": "xxx", "endpoint_url": "...", "max_batch_size": 8192, "max_batch_interval": 2}
func NewAzureProvider() (*AzureProvider, error) {
	instrumentationKey := GetConfigString("azure", "instrumentation_key")
	if instrumentationKey == "" {
		return nil, fmt.Errorf("azure.instrumentation_key not found in LOG_CONFIG")
	}
	
	// Create telemetry client with configurable settings
	telemetryConfig := appinsights.NewTelemetryConfiguration(instrumentationKey)
	telemetryConfig.EndpointUrl = GetConfigString("azure", "endpoint_url", "https://dc.services.visualstudio.com/v2/track")
	telemetryConfig.MaxBatchSize = GetConfigInt("azure", "max_batch_size", 8192)
	telemetryConfig.MaxBatchInterval = time.Duration(GetConfigInt("azure", "max_batch_interval", 2)) * time.Second
	
	return &AzureProvider{
		client: appinsights.NewTelemetryClientFromConfig(telemetryConfig),
	}, nil
}

// Write writes a log entry to Azure Application Insights
func (a *AzureProvider) Write(severity, message string, payload map[string]interface{}) error {
	// Convert severity to Azure severity level
	azureSeverity := parseAzureSeverity(severity)
	
	// Create a trace telemetry
	trace := appinsights.NewTraceTelemetry(message, azureSeverity)
	
	// Add all payload fields as custom properties
	for key, value := range payload {
		trace.Properties[key] = fmt.Sprintf("%v", value)
	}
	
	// Track the trace
	a.client.Track(trace)
	
	return nil
}

// Close flushes and closes the Azure telemetry client
func (a *AzureProvider) Close() error {
	// Flush any pending telemetry
	a.client.Channel().Flush()
	
	// Close the channel
	select {
	case <-a.client.Channel().Close(10): // Wait up to 10 seconds
		return nil
	}
	
	return nil
}

// Name returns the provider name
func (a *AzureProvider) Name() string {
	return "azure"
}

// parseAzureSeverity converts severity string to Azure SeverityLevel
func parseAzureSeverity(severity string) contracts.SeverityLevel {
	switch severity {
	case "DEBUG":
		return contracts.Verbose
	case "INFO":
		return contracts.Information
	case "WARNING":
		return contracts.Warning
	case "ERROR":
		return contracts.Error
	case "CRITICAL":
		return contracts.Critical
	default:
		return contracts.Information
	}
}


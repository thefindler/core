package providers

import (
	"fmt"
	"strings"
	"time"

	"github.com/microsoft/ApplicationInsights-Go/appinsights"
	"github.com/microsoft/ApplicationInsights-Go/appinsights/contracts"
)

// AzureProvider implements logging to Azure Application Insights
type AzureProvider struct {
	client appinsights.TelemetryClient
}

// parseConnectionString extracts InstrumentationKey and IngestionEndpoint from Azure connection string
func parseConnectionString(connStr string) (instrumentationKey, ingestionEndpoint string, err error) {
	parts := strings.Split(connStr, ";")
	for _, part := range parts {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		value := strings.TrimSpace(kv[1])
		
		switch key {
		case "InstrumentationKey":
			instrumentationKey = value
		case "IngestionEndpoint":
			ingestionEndpoint = value
		}
	}
	
	if instrumentationKey == "" {
		return "", "", fmt.Errorf("InstrumentationKey not found in connection string")
	}
	
	return instrumentationKey, ingestionEndpoint, nil
}

// NewAzureProvider creates a new Azure Application Insights logging provider
// Requires LOG_CONFIG["azure"]["connection_string"] in format:
// "InstrumentationKey=xxx;IngestionEndpoint=https://...;ApplicationId=xxx"
func NewAzureProvider() (*AzureProvider, error) {
	connectionString := GetConfigString("azure", "connection_string")
	if connectionString == "" {
		return nil, fmt.Errorf("azure.connection_string not found in LOG_CONFIG")
	}
	
	instrumentationKey, ingestionEndpoint, err := parseConnectionString(connectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse connection string: %w", err)
	}
	
	telemetryConfig := appinsights.NewTelemetryConfiguration(instrumentationKey)
	
	// Set regional ingestion endpoint with required /v2/track path
	if ingestionEndpoint != "" {
		endpoint := strings.TrimSuffix(ingestionEndpoint, "/")
		if !strings.HasSuffix(endpoint, "/v2/track") {
			endpoint = endpoint + "/v2/track"
		}
		telemetryConfig.EndpointUrl = endpoint
	}
	
	telemetryConfig.MaxBatchSize = GetConfigInt("azure", "max_batch_size", 8192)
	telemetryConfig.MaxBatchInterval = time.Duration(GetConfigInt("azure", "max_batch_interval", 2)) * time.Second
	
	return &AzureProvider{
		client: appinsights.NewTelemetryClientFromConfig(telemetryConfig),
	}, nil
}

// Write sends a log entry to Azure Application Insights
func (a *AzureProvider) Write(severity, message string, payload map[string]interface{}) error {
	azureSeverity := parseAzureSeverity(severity)
	trace := appinsights.NewTraceTelemetry(message, azureSeverity)
	
	for key, value := range payload {
		trace.Properties[key] = fmt.Sprintf("%v", value)
	}
	
	a.client.Track(trace)
	return nil
}

// Close flushes and closes the Azure telemetry client
func (a *AzureProvider) Close() error {
	a.client.Channel().Flush()
	
	select {
	case <-a.client.Channel().Close(10 * time.Second):
		return nil
	case <-time.After(10 * time.Second):
		return fmt.Errorf("azure channel close timed out")
	}
}

func (a *AzureProvider) Name() string {
	return "azure"
}

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


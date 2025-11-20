package providers

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// ProviderConfig holds configuration from LOG_CONFIG JSON
var ProviderConfig map[string]interface{}

// Provider defines the interface for logging providers
type Provider interface {
	// Write writes a log entry to the provider
	Write(severity, message string, payload map[string]interface{}) error
	
	// Close closes the provider and flushes any pending writes
	Close() error
	
	// Name returns the provider name for identification
	Name() string
}

// LogEntry represents a structured log entry
type LogEntry struct {
	Severity       string
	Message        string
	Payload        map[string]interface{}
	AgentID        *uuid.UUID
	ConversationID *uuid.UUID
}

// NewProvider creates a new provider based on the provider type
// This follows the factory pattern similar to config/providers/interface.go
func NewProvider(providerType string) (Provider, error) {
	providerType = strings.TrimSpace(strings.ToLower(providerType))
	
	switch providerType {
	case "gcp":
		return NewGCPProvider()
	case "azure":
		return NewAzureProvider()
	case "file":
		return NewFileProvider()
	default:
		return nil, fmt.Errorf("unsupported provider type: %s", providerType)
	}
}

// GetProviderConfig gets the config section for a specific provider
// E.g., for "gcp", returns LOG_CONFIG["gcp"] object
func GetProviderConfig(providerName string) map[string]interface{} {
	if ProviderConfig != nil {
		if cfg, ok := ProviderConfig[providerName].(map[string]interface{}); ok {
			return cfg
		}
	}
	return nil
}

// GetConfigString gets string from LOG_CONFIG with optional default
// Usage: GetConfigString("azure", "key") or GetConfigString("azure", "key", "default")
func GetConfigString(providerName, key string, defaultVal ...string) string {
	if cfg := GetProviderConfig(providerName); cfg != nil {
		if val, ok := cfg[key].(string); ok && val != "" {
			return val
		}
	}
	if len(defaultVal) > 0 {
		return defaultVal[0]
	}
	return ""
}

// GetConfigInt gets int from LOG_CONFIG with optional default
// Usage: GetConfigInt("azure", "key") or GetConfigInt("azure", "key", 8192)
func GetConfigInt(providerName, key string, defaultVal ...int) int {
	if cfg := GetProviderConfig(providerName); cfg != nil {
		if val, ok := cfg[key].(float64); ok {
			return int(val)
		}
		if val, ok := cfg[key].(int64); ok {
			return int(val)
		}
		if val, ok := cfg[key].(int); ok {
			return val
		}
	}
	if len(defaultVal) > 0 {
		return defaultVal[0]
	}
	return 0
}


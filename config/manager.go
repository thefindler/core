package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/thefindler/core/config/providers"
)

// ConfigManager manages configuration from different sources
type ConfigManager struct {
	configSource       string
	provider           providers.ConfigProvider
	fallbackProvider   providers.ConfigProvider
}

// getBootstrapConfig safely retrieves bootstrap configuration values
// This is used before the config system is fully initialized
func getBootstrapConfig(key, defaultValue string) string {
	// Try to get from environment variable first
	if value := os.Getenv(key); value != "" {
		return value
	}
	
	// Try to get from our config system if it's available
	if IsGlobalConfigInitialized() {
		if value := GetConfig(key); value != "" {
			return value
		}
	}
	
	return defaultValue
}

// NewConfigManager creates a new configuration manager
func NewConfigManager() (*ConfigManager, error) {
	// These two environment variables are needed to bootstrap the config system
	// They must be read directly since the config manager isn't available yet
	configSource := os.Getenv("CONFIG_SOURCE")
	if configSource == "" {
		configSource = "env-file" // Default to environment file
	}

	// Read provider-specific configuration only if needed
	var configSourceConfig map[string]interface{}
	if configSource != "env-file" {
		configSourceConfigStr := os.Getenv("CONFIG_SOURCE_CONFIG")
		if configSourceConfigStr != "" {
			if err := json.Unmarshal([]byte(configSourceConfigStr), &configSourceConfig); err != nil {
				return nil, fmt.Errorf("failed to parse CONFIG_SOURCE_CONFIG: %w", err)
			}
		}
	}

	// Create provider factory
	factory := &providers.ProviderFactory{}

	// Create primary provider
	providerConfig := providers.ProviderConfig{
		ProviderType: providers.ProviderType(configSource),
		Config:       configSourceConfig,
	}

	// Create primary provider
	provider, err := factory.NewProvider(providerConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create primary provider: %w", err)
	}

	// Create fallback provider (always env-file)
	fallbackConfig := providers.ProviderConfig{
		ProviderType: providers.ProviderTypeEnvFile,
		Config:       make(map[string]interface{}),
	}

	fallbackProvider, err := factory.NewProvider(fallbackConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create fallback provider: %w", err)
	}

	cm := &ConfigManager{
		configSource:     configSource,
		provider:         provider,
		fallbackProvider: fallbackProvider,
	}


	return cm, nil
}

// Get retrieves a configuration value with proper key normalization
func (cm *ConfigManager) Get(key string) string {
	// The internal get method handles the fallback logic.
	// We pass an empty string as the default value.
	return cm.get(key, "")
}

// GetWithDefault retrieves a configuration value with a specified default.
// It tries the primary provider first, then environment variables, and finally
// returns the default value if the key is not found in any source.
func (cm *ConfigManager) GetWithDefault(key, defaultValue string) string {
	return cm.get(key, defaultValue)
}

// get is the internal method that orchestrates the key retrieval logic.
func (cm *ConfigManager) get(key, defaultValue string) string {
	ctx := context.Background()

	// 1. Determine the search key for the primary provider.
	// For non-env sources, this might involve normalization (e.g., FOO_BAR -> foo-bar).
	searchKey := cm.normalizeKey(key)

	// 2. Try the primary provider.
	value, err := cm.provider.Get(ctx, searchKey)

	// 3. If the primary provider fails (error or empty value), try the fallback.
	// The fallback always uses the original, non-normalized key (e.g., FOO_BAR).
	if err != nil || value == "" {
		// The fallback provider is always the environment file provider.
		value, err = cm.fallbackProvider.Get(ctx, key)
		if err != nil || value == "" {
			// If all sources fail, return the default value.
			return defaultValue
		}
	}

	return value
}

// RefreshCache refreshes the configuration cache
func (cm *ConfigManager) RefreshCache() {
	// This is a generic cache refresh method
	// Specific providers can implement their own caching logic
}

// IsKeyVaultEnabled returns true if Azure Key Vault is the primary provider
func (cm *ConfigManager) IsKeyVaultEnabled() bool {
	return cm.configSource == "azure-keyvault"
}

// GetConfigSource returns the current configuration source
func (cm *ConfigManager) GetConfigSource() string {
	return cm.configSource
}

// normalizeKey normalizes keys based on the configuration source
func (cm *ConfigManager) normalizeKey(key string) string {
	switch cm.configSource {
	case "azure-keyvault":
		// Azure Key Vault doesn't support underscores, use hyphens
		return strings.ReplaceAll(key, "_", "-")
	case "gcp-secretmanager":
		// GCP Secret Manager supports underscores, no change needed
		return key
	case "aws-secretsmanager":
		// AWS Secrets Manager supports underscores, no change needed
		return key
	case "vault":
		// HashiCorp Vault supports underscores, no change needed
		return key
	default:
		// For env-file and unknown providers, no normalization needed
		return key
	}
} 
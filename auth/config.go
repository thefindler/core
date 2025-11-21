package auth

import (
	"fmt"

	"github.com/thefindler/core/config"
)

// JWTConfig holds JWT configuration
type JWTConfig struct {
	Issuer    string
	SecretKey []byte // HMAC secret key for HS256
}

var jwtConfig *JWTConfig

// InitializeJWTConfig initializes JWT configuration from the config system
func InitializeJWTConfig() error {
	secretKey := config.GetConfig("JWT_SECRET_KEY")
	if secretKey == "" {
		return fmt.Errorf("JWT_SECRET_KEY configuration is required")
	}

	jwtConfig = &JWTConfig{
		Issuer:    config.GetConfigWithDefault("JWT_ISSUER", "findler-api-service"),
		SecretKey: []byte(secretKey),
	}

	return nil
}

// GetJWTConfig returns the current JWT configuration
func GetJWTConfig() *JWTConfig {
	return jwtConfig
}


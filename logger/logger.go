package logger

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/thefindler/shared/config"
	"github.com/thefindler/shared/logger/providers"

	"github.com/google/uuid"
)

var (
	// Global logger instance
	globalLogger *Logger
	once         sync.Once
	initErr      error
	
	// Global config from LOG_CONFIG JSON (for providers to use)
	LogConfig map[string]interface{}
)

// Config holds logger configuration
type Config struct {
	Destinations []string
	AsyncEnabled bool
	BufferSize   int
	NumWorkers   int
}

// LogEntry represents a log entry to be processed
type LogEntry struct {
	Severity       string
	Message        string
	Payload        map[string]interface{}
	AgentID        *uuid.UUID
	ConversationID *uuid.UUID
	Error          error
}

// Logger is the main logger with async support
type Logger struct {
	cfg          *Config
	providers    []providers.Provider
	logChannel   chan LogEntry
	workerWg     sync.WaitGroup
	shutdown     chan struct{}
	shutdownOnce sync.Once
}

// InitLogger initializes the global logger instance
// This should be called once at application startup
func InitLogger() error {
	once.Do(func() {
		globalLogger, initErr = NewLogger()
	})
	return initErr
}

// NewLogger creates a new logger instance with configured providers
func NewLogger() (*Logger, error) {
	// Parse LOG_CONFIG JSON from config manager
	if logConfigStr := config.GetConfig("LOG_CONFIG"); logConfigStr != "" {
		json.Unmarshal([]byte(logConfigStr), &LogConfig)
	}
	if LogConfig == nil {
		LogConfig = make(map[string]interface{})
	}
	
	// Simple inline config loading
	destinations := getDestinations()
	asyncEnabled := config.GetConfig("LOG_ASYNC") != "false" // default true
	bufferSize := getConfigInt("LOG_BUFFER_SIZE", 1000)
	numWorkers := getConfigInt("LOG_NUM_WORKERS", 3)
	
	cfg := &Config{
		Destinations: destinations,
		AsyncEnabled: asyncEnabled,
		BufferSize:   bufferSize,
		NumWorkers:   numWorkers,
	}
	
	// Pass config to providers
	providers.ProviderConfig = LogConfig
	
	// Initialize providers
	providersList := make([]providers.Provider, 0, len(cfg.Destinations))
	for _, dest := range cfg.Destinations {
		if provider, err := providers.NewProvider(dest); err == nil {
			providersList = append(providersList, provider)
			log.Printf("✓ Logger provider: %s", dest)
		}
	}
	
	if len(providersList) == 0 {
		return nil, fmt.Errorf("no logging providers initialized")
	}
	
	l := &Logger{
		cfg:       cfg,
		providers: providersList,
		shutdown:  make(chan struct{}),
	}
	
	if cfg.AsyncEnabled {
		l.logChannel = make(chan LogEntry, cfg.BufferSize)
		l.startWorkers()
	}
	
	return l, nil
}

// getDestinations parses LOG_DESTINATION from config
func getDestinations() []string {
	dest := config.GetConfig("LOG_DESTINATION")
	if dest == "" {
		if config.GetConfig("ENV") == "development" {
			return []string{"file"}
		}
		return []string{"gcp"}
	}
	if dest == "both" { // backward compat
		return []string{"gcp", "file"}
	}
	parts := strings.Split(dest, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			result = append(result, p)
		}
	}
	return result
}

func getConfigInt(key string, def int) int {
	if val := config.GetConfig(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return def
}

// startWorkers starts background workers to process log entries
func (l *Logger) startWorkers() {
	for i := 0; i < l.cfg.NumWorkers; i++ {
		l.workerWg.Add(1)
		go l.worker(i)
	}
}

// worker processes log entries from the channel
func (l *Logger) worker(workerID int) {
	defer l.workerWg.Done()
	
	for {
		select {
		case entry := <-l.logChannel:
			// Write to all providers
			l.writeToProviders(entry)
			
		case <-l.shutdown:
			// Drain remaining logs before shutdown
			for len(l.logChannel) > 0 {
				entry := <-l.logChannel
				l.writeToProviders(entry)
			}
			return
		}
	}
}

// writeToProviders writes a log entry to all configured providers
func (l *Logger) writeToProviders(entry LogEntry) {
	for _, p := range l.providers {
		if err := p.Write(entry.Severity, entry.Message, entry.Payload); err != nil {
			// Log error to stderr without blocking
			fmt.Fprintf(os.Stderr, "[Logger Error] Provider %s failed: %v\n", p.Name(), err)
		}
	}
}

// log is the internal logging function
func (l *Logger) log(severity, message string, data map[string]interface{}, 
	agentID, conversationID *uuid.UUID, err error) {
	
	// Apply PII masking if enabled (read from config at runtime)
	maskedMessage := message
	maskedData := data
	
	piiMaskingEnabled := config.GetConfig("LOG_PII_MASKING") != "false" // default true
	if piiMaskingEnabled {
		maskedMessage = maskPIIString(message)
		maskedData = maskPIIData(data)
	}
	
	// Build payload
	payload := make(map[string]interface{})
	payload["message"] = maskedMessage
	
	if maskedData != nil {
		for key, value := range maskedData {
			payload[key] = value
		}
	}
	
	if agentID != nil {
		payload["agent_id"] = agentID.String()
	}
	
	if conversationID != nil {
		payload["conversation_id"] = conversationID.String()
	}
	
	if err != nil {
		errorStr := fmt.Sprintf("%v", err)
		if piiMaskingEnabled {
			errorStr = maskPIIString(errorStr)
		}
		payload["error"] = errorStr
	}
	
	// Console output in development mode (immediate, not buffered)
	if config.GetConfig("ENV") == "development" {
		color := getSeverityColorCode(severity)
		log.Printf("[%s%s\033[0m] %s", color, severity, maskedMessage)
	}
	
	entry := LogEntry{
		Severity:       severity,
		Message:        maskedMessage,
		Payload:        payload,
		AgentID:        agentID,
		ConversationID: conversationID,
		Error:          err,
	}
	
	// Write async or sync based on configuration
	if l.cfg.AsyncEnabled {
		// Non-blocking send to channel
		select {
		case l.logChannel <- entry:
			// Successfully queued
		default:
			// Buffer full - emergency fallback to file provider
			for _, p := range l.providers {
				if p.Name() == "file" {
					p.Write(entry.Severity, entry.Message, entry.Payload)
					break
				}
			}
			
			// Log to stderr
			fmt.Fprintf(os.Stderr, "[CRITICAL] Log buffer full! Dropped log: %s\n", maskedMessage)
		}
	} else {
		// Synchronous write
		l.writeToProviders(entry)
	}
}

// Shutdown gracefully shuts down the logger, flushing all pending logs
func (l *Logger) Shutdown() {
	l.shutdownOnce.Do(func() {
		if l.cfg.AsyncEnabled {
			// Signal workers to shutdown
			close(l.shutdown)
			
			// Wait for workers to finish
			l.workerWg.Wait()
			
			// Close channel
			close(l.logChannel)
		}
		
		// Close all providers
		for _, p := range l.providers {
			if err := p.Close(); err != nil {
				log.Printf("Error closing provider %s: %v", p.Name(), err)
			}
		}
		
		log.Println("✓ Logger shutdown complete")
	})
}

// Public API functions (compatible with existing api/logger interface)

// Info logs an informational message
func Info(message string, data map[string]interface{}, agentID *uuid.UUID, conversationID *uuid.UUID) {
	if globalLogger != nil {
		globalLogger.log("INFO", message, data, agentID, conversationID, nil)
	}
}

// Error logs an error message
func Error(message string, data map[string]interface{}, agentID *uuid.UUID, conversationID *uuid.UUID, err error) {
	if globalLogger != nil {
		globalLogger.log("ERROR", message, data, agentID, conversationID, err)
	}
}

// Warn logs a warning message
func Warn(message string, data map[string]interface{}, agentID *uuid.UUID, conversationID *uuid.UUID, err error) {
	if globalLogger != nil {
		globalLogger.log("WARNING", message, data, agentID, conversationID, err)
	}
}

// Critical logs a critical error message
func Critical(message string, data map[string]interface{}, agentID *uuid.UUID, conversationID *uuid.UUID, err error) {
	if globalLogger != nil {
		globalLogger.log("CRITICAL", message, data, agentID, conversationID, err)
	}
}

// Debug logs a debug message
func Debug(message string, data map[string]interface{}, agentID *uuid.UUID, conversationID *uuid.UUID) {
	if globalLogger != nil {
		globalLogger.log("DEBUG", message, data, agentID, conversationID, nil)
	}
}

// Shutdown gracefully shuts down the global logger
func Shutdown() {
	if globalLogger != nil {
		globalLogger.Shutdown()
	}
}

// LogPII executes the provided logging function only if LOG_PII is enabled
// Copied from existing api/logger/index.go for compatibility
func LogPII(logFunc func()) {
	if config.GetConfig("LOG_PII") == "true" {
		logFunc()
	}
}

// MergeLogdata merges multiple log data maps
// Copied from existing api/logger/index.go for compatibility
func MergeLogdata(maps ...map[string]interface{}) map[string]interface{} {
	merged := make(map[string]interface{})
	for _, m := range maps {
		for k, v := range m {
			merged[k] = v
		}
	}
	return merged
}

// getSeverityColorCode returns ANSI color code for severity
func getSeverityColorCode(severity string) string {
	switch severity {
	case "DEBUG":
		return "\033[36m" // Cyan
	case "INFO":
		return "\033[32m" // Green
	case "WARNING":
		return "\033[33m" // Yellow
	case "ERROR":
		return "\033[31m" // Red
	case "CRITICAL":
		return "\033[35m" // Magenta
	default:
		return "\033[0m"
	}
}


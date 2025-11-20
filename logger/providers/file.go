package providers

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// FileProvider implements file-based logging with rotation
type FileProvider struct {
	filePath    string
	maxFileSize int64
	logFile     *os.File
	mutex       sync.Mutex
}

// NewFileProvider creates a new file logging provider
// Config from LOG_CONFIG["file"]: {"log_file_path": "...", "max_file_size": 104857600}
func NewFileProvider() (*FileProvider, error) {
	filePath := GetConfigString("file", "log_file_path", "./logs/api.log")
	maxFileSize := int64(GetConfigInt("file", "max_file_size", 100*1024*1024))
	
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return nil, err
	}

	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	return &FileProvider{
		filePath:    filePath,
		maxFileSize: maxFileSize,
		logFile:     file,
	}, nil
}

// Write writes a log entry to file
func (fp *FileProvider) Write(severity, message string, payload map[string]interface{}) error {
	fp.mutex.Lock()
	defer fp.mutex.Unlock()

	fp.rotate()

	timestamp := time.Now().Format("2006-01-02T15:04:05.000Z07:00")
	var dataStr string
	if len(payload) > 0 {
		pairs := make([]string, 0, len(payload))
		for k, v := range payload {
			pairs = append(pairs, fmt.Sprintf("%s=%v", k, v))
		}
		dataStr = " " + strings.Join(pairs, " ")
	}

	logLine := fmt.Sprintf("[%s] [%s] %s%s\n", timestamp, strings.ToUpper(severity), message, dataStr)
	_, err := fp.logFile.WriteString(logLine)
	return err
}

// Close closes the file logger
func (fp *FileProvider) Close() error {
	fp.mutex.Lock()
	defer fp.mutex.Unlock()
	if fp.logFile != nil {
		return fp.logFile.Close()
	}
	return nil
}

// Name returns the provider name
func (fp *FileProvider) Name() string {
	return "file"
}

// rotate rotates log file if size exceeds limit
func (fp *FileProvider) rotate() {
	info, err := fp.logFile.Stat()
	if err != nil || info.Size() < fp.maxFileSize {
		return
	}

	fp.logFile.Close()
	
	// Simple rotation: rename current to .old, create new
	oldPath := fp.filePath + ".old"
	os.Remove(oldPath) // Remove old backup if exists
	os.Rename(fp.filePath, oldPath)
	
	fp.logFile, _ = os.OpenFile(fp.filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
}


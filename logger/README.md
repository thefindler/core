# Shared Logger

World-class logging system with async support, PII masking, and multiple providers (GCP, Azure, File).

## Configuration

### Simple: Only 2 Environment Variables

```bash
# Which providers to use
LOG_DESTINATION=gcp,azure,file

# All provider configs in one JSON
LOG_CONFIG='{
  "gcp": {
    "project_id": "my-gcp-project",
    "project_name": "findler",
    "env": "production"
  },
  "azure": {
    "instrumentation_key": "your-azure-instrumentation-key"
  },
  "file": {
    "log_file_path": "./logs/api.log",
    "max_file_size": 104857600
  }
}'
```

### Backward Compatible

Falls back to individual env vars if not in LOG_CONFIG:
- `GOOGLE_PROJECT_ID`, `PROJECT`, `ENV` for GCP
- `AZURE_APPINSIGHTS_KEY` for Azure  
- `LOG_FILE_PATH`, `LOG_MAX_FILE_SIZE` for File

### Optional Settings

```bash
# Enable/disable async logging (default: true)
LOG_ASYNC=true

# Buffer size for async logging (default: 1000)
LOG_BUFFER_SIZE=1000

# Number of worker goroutines (default: 3)
LOG_NUM_WORKERS=3

# Enable PII masking for ISO 27001 compliance (default: true)
LOG_PII_MASKING=true
```

## Usage

```go
import "github.com/thefindler/shared/logger"

// Initialize once at startup
logger.InitLogger()
defer logger.Shutdown() // Gracefully flush logs

// Use anywhere
logger.Info("User logged in", map[string]interface{}{
    "user_id": "123",
}, &agentID, &conversationID)

logger.Error("Failed to process", data, &agentID, &conversationID, err)
```

## Features

- ✅ **Async logging** - Non-blocking with buffered channels
- ✅ **Multiple providers** - GCP, Azure, File (easily extensible)
- ✅ **PII masking** - Automatic masking of sensitive data (Aadhaar, PAN, phone, email, etc.)
- ✅ **OpenTelemetry metrics** - Monitor buffer size, dropped logs, etc.
- ✅ **Graceful shutdown** - Flushes all pending logs
- ✅ **Clean configuration** - All logging config in LOG_CONFIG JSON


package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/thefindler/core/logger"

	"cloud.google.com/go/storage"
)

var (
	clientOnce sync.Once
	gcsClient  *storage.Client
	clientErr  error
)

// gcsFileStorage implements FileStorage interface for Google Cloud Storage
type gcsFileStorage struct{}

// NewGCSFileStorage creates a new GCS file storage instance.
// Uses singleton pattern - all instances share the same underlying GCS client.
//
// Authentication: Uses Google Application Default Credentials (ADC) which automatically
// detects credentials in the following order:
// 1. GOOGLE_APPLICATION_CREDENTIALS environment variable (path to service account key file)
// 2. gcloud CLI credentials (via `gcloud auth application-default login`)
// 3. GCE metadata service (if running on Google Cloud Platform)
//
// Example: Set GOOGLE_APPLICATION_CREDENTIALS=/path/to/service-account-key.json
func NewGCSFileStorage() FileStorage {
	return &gcsFileStorage{}
}

// getClient initializes (once) and returns a shared *storage.Client.
// Uses Application Default Credentials for authentication.
func getClient(ctx context.Context) (*storage.Client, error) {
	clientOnce.Do(func() {
		gcsClient, clientErr = storage.NewClient(ctx)
		if clientErr != nil {
			logger.Error(
				"GCS Storage: Failed to create storage client",
				nil,
				nil,
				nil,
				clientErr,
			)
		} else {
			logger.Info("GCS Storage: Storage client initialized", nil, nil, nil)
		}
	})
	return gcsClient, clientErr
}

// UploadFile uploads the local file at localPath to the given GCS bucket/object.
// If objectName is empty, the base filename of localPath is used.
// After successful upload, the local file is deleted.
func (g *gcsFileStorage) UploadFile(ctx context.Context, bucket, objectName, localPath string) error {
	if bucket == "" {
		return fmt.Errorf("bucket cannot be empty")
	}
	if objectName == "" {
		objectName = filepath.Base(localPath)
	}

	client, err := getClient(ctx)
	if err != nil {
		logger.Error(
			"GCS Storage: Failed to get storage client",
			map[string]interface{}{"bucket": bucket},
			nil,
			nil,
			err,
		)
		return fmt.Errorf("get storage client: %w", err)
	}

	f, err := os.Open(localPath)
	if err != nil {
		logger.Error(
			"GCS Storage: Failed to open local file",
			map[string]interface{}{"local_path": localPath},
			nil,
			nil,
			err,
		)
		return fmt.Errorf("open local file: %w", err)
	}
	defer f.Close()

	wc := client.Bucket(bucket).Object(objectName).NewWriter(ctx)
	if _, err := io.Copy(wc, f); err != nil {
		wc.Close()
		logger.Error(
			"GCS Storage: Copy to GCS failed",
			map[string]interface{}{"bucket": bucket, "object": objectName},
			nil,
			nil,
			err,
		)
		return fmt.Errorf("copy to GCS: %w", err)
	}
	if err := wc.Close(); err != nil {
		logger.Error(
			"GCS Storage: Closing writer failed",
			map[string]interface{}{"bucket": bucket, "object": objectName},
			nil,
			nil,
			err,
		)
		return fmt.Errorf("closing GCS writer: %w", err)
	}

	// Remove local file after successful upload
	if err := os.Remove(localPath); err != nil {
		logger.Warn(
			"GCS Storage: Uploaded but failed to remove local file",
			map[string]interface{}{"path": localPath},
			nil,
			nil,
			err,
		)
		return nil
	}

	return nil
}

// Close releases resources associated with the shared client.
// Should be called during graceful shutdown.
func (g *gcsFileStorage) Close() error {
	if gcsClient != nil {
		logger.Info("GCS Storage: Closing shared storage client", nil, nil, nil)
		return gcsClient.Close()
	}
	return nil
}

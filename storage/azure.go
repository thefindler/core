package storage

import (
	"bytes"
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
)

// BlobStorage defines the interface for object storage
type BlobStorage interface {
	Upload(ctx context.Context, container, blobName string, data []byte) error
	Download(ctx context.Context, container, blobName string) ([]byte, error)
	Exists(ctx context.Context, container, blobName string) (bool, error)
	EnsureContainer(ctx context.Context, container string) error
}

type azureBlobStorage struct {
	client *azblob.Client
}

// NewAzureBlobStorage creates a new Azure Blob Storage client
// connectionString: Azure Storage Connection String
func NewAzureBlobStorage(connectionString string) (BlobStorage, error) {
	client, err := azblob.NewClientFromConnectionString(connectionString, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create azure blob client: %w", err)
	}

	return &azureBlobStorage{
		client: client,
	}, nil
}

func (s *azureBlobStorage) Upload(ctx context.Context, container, blobName string, data []byte) error {
	_, err := s.client.UploadBuffer(ctx, container, blobName, data, nil)
	return err
}

func (s *azureBlobStorage) Download(ctx context.Context, container, blobName string) ([]byte, error) {
	downloadResponse, err := s.client.DownloadStream(ctx, container, blobName, nil)
	if err != nil {
		return nil, err
	}
	defer downloadResponse.Body.Close()

	// Read all into buffer
	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(downloadResponse.Body)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (s *azureBlobStorage) Exists(ctx context.Context, container, blobName string) (bool, error) {
	// Simple way to check existence is to get properties
	// If it fails with 404, it doesn't exist
	_, err := s.client.ServiceClient().NewContainerClient(container).NewBlobClient(blobName).GetProperties(ctx, nil)
	if err != nil {
		// TODO: Check strictly for 404 if needed, but for now error means "probably not there"
		return false, nil
	}
	return true, nil
}

// EnsureContainer creates a container if it doesn't exist
func (s *azureBlobStorage) EnsureContainer(ctx context.Context, container string) error {
	containerClient := s.client.ServiceClient().NewContainerClient(container)
	_, err := containerClient.Create(ctx, nil)
	if err != nil {
		// Check if error is because container already exists
		// Azure returns an error even if container exists, so we check properties
		_, existsErr := containerClient.GetProperties(ctx, nil)
		if existsErr == nil {
			// Container already exists, this is fine
			return nil
		}
		return fmt.Errorf("failed to create container: %w", err)
	}
	return nil
}



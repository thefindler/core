package storage

import "context"

// FileStorage defines the interface for file-based object storage operations.
// This is separate from BlobStorage which handles byte-based operations.
type FileStorage interface {
	// UploadFile uploads a local file to the storage bucket.
	// After successful upload, the local file is deleted.
	// bucket: The bucket/container name
	// objectName: The object/blob name in the bucket (path within bucket)
	// localPath: The local file system path to the file to upload
	UploadFile(ctx context.Context, bucket, objectName, localPath string) error

	// Close releases resources associated with the storage client.
	// Should be called during graceful shutdown.
	Close() error
}

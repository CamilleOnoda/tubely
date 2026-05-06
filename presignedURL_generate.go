package main

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// generatePresignedURL generates a presigned URL for the given S3 bucket
//
// and key with the specified expiration time.
func generatePresignedURL(
	s3Client *s3.Client,
	bucket,
	key string,
	expireTime time.Duration) (string, error) {
	presignClient := s3.NewPresignClient(s3Client)
	presignedURL, err := presignClient.PresignGetObject(context.Background(), &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}, s3.WithPresignExpires(expireTime))
	if err != nil {
		return "", err
	}

	return presignedURL.URL, nil
}

package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/database"
)

// dbVideoToSignedVideo takes a database.Video
// and returns a new database.Video
//
// with the VideoURL field updated to a presigned URL if the original VideoURL is not nil.
func (cfg *apiConfig) dbVideoToSignedVideo(video database.Video) (database.Video, error) {
	if video.VideoURL == nil {
		return video, nil
	}

	split := strings.SplitN(*video.VideoURL, ",", 2)
	if len(split) != 2 {
		err := fmt.Errorf("invalid stored video URL format: %s", *video.VideoURL)
		return video, err
	}
	bucket := split[0]
	key := split[1]

	expireTime := time.Minute * 5
	presignedURL, err := generatePresignedURL(cfg.s3Client, bucket, key, expireTime)
	if err != nil {
		return video, err
	}

	video.VideoURL = &presignedURL

	return video, nil
}

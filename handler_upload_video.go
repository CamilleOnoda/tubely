package main

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

// handlerUploadVideo handles the video upload process.
//
// It validates the request, checks the user's authentication,
// determines the video's aspect ratio, processes the video for fast start streaming,
// uploads the video to S3, and updates the video's metadata in the database with the S3 URL.
func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<30)
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest,
			"Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized,
			"Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized,
			"Couldn't validate JWT", err)
		return
	}

	fmt.Println("uploading video for video", videoID, "by user", userID)

	if err := r.ParseMultipartForm(1 << 30); err != nil {
		respondWithError(w, http.StatusBadRequest,
			"Can't process the request due to malformed syntax", err)
		return
	}

	srcFile, header, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest,
			"Unable to parse file", err)
		return
	}
	defer srcFile.Close()

	contentType := header.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest,
			"media type should be video/mp4", err)
		return
	}

	videoDB, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError,
			"Failed to retrieve the video's metadata", err)
		return
	}
	if videoDB.UserID != userID {
		respondWithError(w, http.StatusUnauthorized,
			"The authenticated user is not the video owner", err)
		return
	}

	tempFile, err := os.CreateTemp("", "tubely-upload-*")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError,
			"Unable to create temporary file", err)
		return
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	_, err = io.Copy(tempFile, srcFile)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError,
			"Unable to save the uploaded file", err)
		return
	}
	tempFile.Seek(0, io.SeekStart)

	aspectRatio, err := GetVideoAspectRatio(tempFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError,
			"Unable to determine video aspect ratio", err)
		return
	}

	switch aspectRatio {
	case "16:9":
		videoIDString = "landscape/" + videoIDString
	case "9:16":
		videoIDString = "portrait/" + videoIDString
	default:
		videoIDString = "other/" + videoIDString
	}

	processedOutput, err := ProcessVideoForFastStart(tempFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError,
			"Unable to process the video for fast start streaming", err)
		return
	}

	processedFile, err := os.Open(processedOutput)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError,
			"Unable to open the processed video file", err)
		return
	}
	defer processedFile.Close()

	_, err = cfg.s3Client.PutObject(r.Context(), &s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &videoIDString,
		Body:        processedFile,
		ContentType: &contentType,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError,
			"Unable to upload the file to S3", err)
		return
	}

	storedURL := fmt.Sprintf("%s,%s", cfg.s3Bucket, videoIDString)
	videoDB.VideoURL = &storedURL
	err = cfg.db.UpdateVideo(videoDB)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError,
			"Could not update the video with the video URL", err)
		return
	}

	presignedVideoURL, err := cfg.dbVideoToSignedVideo(videoDB)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError,
			"Unable to generate a presigned URL for the uploaded video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, presignedVideoURL)

}

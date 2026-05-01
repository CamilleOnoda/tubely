package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"os/exec"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

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

	metaData, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError,
			"Failed to retrieve the video's metadata", err)
		return
	}
	if metaData.UserID != userID {
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

	aspectRatio, err := getVideoAspectRatio(tempFile.Name())
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

	_, err = cfg.s3Client.PutObject(r.Context(), &s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &videoIDString,
		Body:        tempFile,
		ContentType: &contentType,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError,
			"Unable to upload the file to S3", err)
		return
	}

	url := fmt.Sprintf("https://%v.s3.%v.amazonaws.com/%v", cfg.s3Bucket, cfg.s3Region, videoIDString)
	metaData.VideoURL = &url
	err = cfg.db.UpdateVideo(metaData)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError,
			"Could not update the video with the video URL", err)
		return
	}

	respondWithJSON(w, http.StatusOK, metaData)

}

// getVideoAspectRatio uses ffprobe to determine the aspect ratio of a video file at the given filePath.
// It returns a string representing the aspect ratio ("16:9", "9:16", or "other") and an error if any occurs.
func getVideoAspectRatio(filePath string) (string, error) {
	cmd := exec.Command(
		"ffprobe",
		"-v",
		"error",
		"-print_format",
		"json",
		"-show_streams",
		filePath,
	)

	var outBuffer bytes.Buffer
	cmd.Stdout = &outBuffer

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to execute ffprobe: %w", err)
	}

	unmarshalledOutput := struct {
		Streams []struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"streams"`
	}{}
	if err := json.Unmarshal(outBuffer.Bytes(), &unmarshalledOutput); err != nil {
		return "", fmt.Errorf("failed to unmarshal ffprobe output: %w", err)
	}
	if len(unmarshalledOutput.Streams) == 0 {
		return "", fmt.Errorf("no streams found in ffprobe output")
	}

	width := unmarshalledOutput.Streams[0].Width
	height := unmarshalledOutput.Streams[0].Height
	if width == 0 || height == 0 {
		return "", fmt.Errorf("invalid width or height in ffprobe output")
	}

	const tolerance = 0.02 // Allow 2% tolerance for aspect ratio comparison
	ratio := float64(width) / float64(height)
	if abs(ratio-16.0/9.0) < tolerance {
		return "16:9", nil
	}
	if abs(ratio-9.0/16.0) < tolerance {
		return "9:16", nil
	}

	return "other", nil
}

// abs returns the absolute value of a float64 number.
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

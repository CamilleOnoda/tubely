package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	maxMemory := 10 << 20
	if err := r.ParseMultipartForm(int64(maxMemory)); err != nil {
		respondWithError(w, http.StatusBadRequest,
			"Can't process the request due to malformed syntax", err)
		return
	}

	srcFile, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse file", err)
		return
	}
	defer srcFile.Close()

	contentType := header.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || (mediaType != "image/jpeg" && mediaType != "image/png") {
		respondWithError(w, http.StatusBadRequest,
			"media type should be either image/jpeg or image/png", err)
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

	fileExtension := strings.Split(mediaType, "/")
	byteSlice := make([]byte, 32)
	rand.Read(byteSlice)
	encoded := base64.RawURLEncoding.EncodeToString(byteSlice)
	filePath := filepath.Join(cfg.assetsRoot, encoded+"."+fileExtension[1])
	newFile, err := os.Create(filePath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError,
			"Failed to create file path", err)
		return
	}

	_, err = io.Copy(newFile, srcFile)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError,
			"Failed to copy the file to disk", err)
		return
	}

	url := fmt.Sprintf("http://localhost:%v/assets/%v.%v", cfg.port, encoded, fileExtension[1])
	metaData.ThumbnailURL = &url

	err = cfg.db.UpdateVideo(metaData)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError,
			"Could not update the video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, metaData)
}

package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)

	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid video ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT Token", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT token", err)
		return
	}

	log.Println("Uploading video:", videoID, "by user:", userID)

	const maxMemory = 10 << 30

	r.ParseMultipartForm(maxMemory)
	file, header, err := r.FormFile("video")

	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Error parsing video file", err)
		return
	}

	defer file.Close()

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusForbidden, "You are not permitted to modify this file", err)
		return
	}

	contentType, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Error parsing the content type", err)
		return
	}

	if contentType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "Can only accept video files", err)
		return
	}

	f, err := os.CreateTemp("", "tubely-upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error creating a temporary file", err)
		return
	}

	defer os.Remove(f.Name())
	defer f.Close()

	_, err = io.Copy(f, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error creating file", err)
		return
	}

	f.Seek(0, io.SeekStart)

	layout, err := getVideoAspectRatio(f.Name())
	log.Println(layout)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error getting the aspect ratio", err)
		return
	}

	switch layout {
	case "16:9":
		layout = "landscape"
	case "9:16":
		layout = "portrait"
	default:
		layout = "other"
	}

	extension := strings.Split(contentType, "/")[1]
	key := make([]byte, 32)
	// fill key with cryptographic secure characters
	rand.Read(key)
	nameFile := base64.RawURLEncoding.EncodeToString(key)
	fileName := fmt.Sprintf("%s/%s.%s", layout, nameFile, extension)
	processedFilePath, err := processVideoForFastStart(f.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error processing the video file", err)
		return
	}

	processedFile, err := os.Open(processedFilePath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Can't open the processed file", err)
		return
	}
	defer processedFile.Close()

	// put files into s3 bucket
	log.Println("Uploading video", fileName, "to s3 bucket")

	_, err = cfg.s3Client.PutObject(
		r.Context(),
		&s3.PutObjectInput{
			Bucket:      &cfg.s3Bucket,
			Key:         &fileName,
			Body:        processedFile,
			ContentType: &contentType,
		})

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error uploading file to s3 bucket", err)
		return
	}

	log.Printf("Finished uploading video: %v to s3 bucket", fileName)

	videoURL := fmt.Sprintf("%s,%s", cfg.s3Bucket, fileName)

	video.VideoURL = &videoURL

	if err = cfg.db.UpdateVideo(video); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error updating video in database", err)
	}

	signedVideo, err := cfg.dbVideoToSignedVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Can not sign the video", err)
		return
	}
	respondWithJSON(w, 200, signedVideo)
}

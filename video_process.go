package main

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/database"
)

func processVideoForFastStart(filePath string) (string, error) {
	outputPath := filePath + ".processing"

	cmd := exec.Command("ffmpeg", "-i", filePath, "-c", "copy", "-movflags", "faststart", "-f", "mp4", outputPath)

	if err := cmd.Run(); err != nil {
		return "", err
	}

	return outputPath, nil
}

func generatePresignedURL(s3Client *s3.Client, bucket, key string, expireTime time.Duration) (string, error) {
	client := s3.NewPresignClient(s3Client)
	ctx := context.Background()

	presignReq, err := client.PresignGetObject(
		ctx,
		&s3.GetObjectInput{
			Bucket: &bucket,
			Key:    &key,
		},
		s3.WithPresignExpires(expireTime),
	)
	if err != nil {
		return "", err
	}
	return presignReq.URL, nil
}

func (cfg *apiConfig) dbVideoToSignedVideo(video database.Video) (database.Video, error) {
	bucket, key, found := strings.Cut(aws.StringValue(video.VideoURL), ",")

	log.Println(bucket, key)
	if !found {
		return video, fmt.Errorf("video url is not correctly formatted")
	}

	expireTime := 1 * time.Hour
	url, err := generatePresignedURL(cfg.s3Client, bucket, key, expireTime)

	if err != nil {
		return video, err
	}
	video.VideoURL = &url
	return video, nil
}

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os/exec"
)

func respondWithError(w http.ResponseWriter, code int, msg string, err error) {
	if err != nil {
		log.Println(err)
	}
	if code > 499 {
		log.Printf("Responding with 5XX error: %s", msg)
	}
	type errorResponse struct {
		Error string `json:"error"`
	}
	respondWithJSON(w, code, errorResponse{
		Error: msg,
	})
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	dat, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		w.WriteHeader(500)
		return
	}
	w.WriteHeader(code)
	w.Write(dat)
}

type ffprobeOutputJson struct {
	Streams []struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	} `json:"streams"`
}

func getVideoAspectRatio(filePath string) (string, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filePath)
	b := bytes.Buffer{}
	cmd.Stdout = &b

	if err := cmd.Run(); err != nil {
		return "", err
	}

	data := ffprobeOutputJson{}
	if err := json.Unmarshal(b.Bytes(), &data); err != nil {
		return "", err
	}

	if len(data.Streams) == 0 || data.Streams[0].Height == 0 {
		return "", fmt.Errorf("invalid video dimensions")
	}

	width := float64(data.Streams[0].Width)
	height := float64(data.Streams[0].Height)

	rawRatio := width / height

	roundedRatio := math.Round(rawRatio*100) / 100

	switch roundedRatio {
	case 1.78:
		return "16:9", nil
	case 0.56:
		return "9:16", nil
	default:
		return "other", nil
	}
}

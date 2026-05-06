package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
)

// getVideoAspectRatio uses ffprobe to determine
// the aspect ratio of a video file at the given filePath.
//
// It returns a string representing the aspect ratio
// ("16:9", "9:16", or "other") and an error if any occurs.
func GetVideoAspectRatio(filePath string) (string, error) {
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

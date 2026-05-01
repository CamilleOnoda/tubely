package main

import (
	"fmt"
	"os/exec"
)

// ProcessVideoForFastStart uses ffmpeg to process the video
// at the given filePath for fast start streaming.
//
// It returns the file path of the processed video and an error if any occurs.
func ProcessVideoForFastStart(filePath string) (string, error) {
	processedOutput := filePath + "_faststart.mp4"
	cmd := exec.Command(
		"ffmpeg",
		"-i",
		filePath,
		"-c",
		"copy",
		"-movflags",
		"faststart",
		"-f",
		"mp4",
		processedOutput,
	)

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to execute ffmpeg: %w", err)
	}

	return processedOutput, nil
}

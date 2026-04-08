package services

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
)

// MediaInfo contains basic metadata discovered during pre-check
type MediaInfo struct {
	IsIndexable bool
	Format      string
	Duration    float64
}

// ValidateMedia checks if a file is supported and readable by ffmpeg/AI
func ValidateMedia(ctx context.Context, filePath string, category string) MediaInfo {
	info := MediaInfo{IsIndexable: false}

	switch category {
	case "photo":
		// Standard image formats supported by Gemini
		info.IsIndexable = true
		info.Format = "image"
	case "document":
		// Check for PDF extension as a proxy for Gemini PDF support
		if strings.HasSuffix(strings.ToLower(filePath), ".pdf") {
			info.IsIndexable = true
			info.Format = "pdf"
		}
	case "video", "audio":
		// Use ffprobe to verify the file is not corrupted and get duration
		cmd := exec.CommandContext(ctx, "ffprobe", 
			"-v", "error", 
			"-show_entries", "format=duration,format_name", 
			"-of", "default=noprint_wrappers=1:nokey=1", 
			filePath)
		
		output, err := cmd.CombinedOutput()
		if err != nil {
			log.Printf("⚠️ Media validation failed for %s: %v", filePath, err)
			return info
		}

		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		if len(lines) >= 2 {
			info.Format = lines[0] // e.g., mov,mp4,m4a,3gp,3g2,mj2
			fmt.Sscanf(lines[1], "%f", &info.Duration)
			info.IsIndexable = true
		}
	}

	return info
}

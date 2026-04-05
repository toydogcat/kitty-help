package services

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	GeminiEmbeddingModel = "gemini-embedding-2-preview"
	GeminiAPIBase        = "https://generativelanguage.googleapis.com/v1beta"
)

type EmbeddingResponse struct {
	Embedding struct {
		Values []float32 `json:"values"`
	} `json:"embedding"`
}

// GenerateMultimodalEmbedding handles text, images, audio, video, and PDF
func GenerateMultimodalEmbedding(ctx context.Context, apiKey string, filePath string, category string, text string) ([]float32, error) {
	parts := []map[string]interface{}{}

	if text != "" {
		parts = append(parts, map[string]interface{}{
			"text": text,
		})
	}

	if filePath != "" {
		var mimeType string
		var data []byte
		var err error

		switch category {
		case "photo":
			mimeType = "image/jpeg"
			data, err = os.ReadFile(filePath)
		case "audio":
			mimeType = "audio/mpeg"
			data, err = os.ReadFile(filePath)
		case "document":
			mimeType = "application/pdf"
			data, err = os.ReadFile(filePath)
		case "video":
			mimeType = "video/mp4"
			trimmedPath := filepath.Join(os.TempDir(), fmt.Sprintf("trim_%s.mp4", filepath.Base(filePath)))
			cmd := exec.CommandContext(ctx, "ffmpeg", "-y", "-i", filePath, "-t", "15", "-c", "copy", trimmedPath)
			if err := cmd.Run(); err != nil {
				log.Printf("⚠️ FFmpeg trim failed: %v", err)
				data, err = os.ReadFile(filePath)
			} else {
				data, err = os.ReadFile(trimmedPath)
				os.Remove(trimmedPath)
			}
		}

		if err == nil && len(data) > 0 {
			parts = append(parts, map[string]interface{}{
				"inline_data": map[string]interface{}{
					"mime_type": mimeType,
					"data":      base64.StdEncoding.EncodeToString(data),
				},
			})
		}
	}

	url := fmt.Sprintf("%s/models/%s:embedContent?key=%s", GeminiAPIBase, GeminiEmbeddingModel, apiKey)
	payload := map[string]interface{}{
		"model": fmt.Sprintf("models/%s", GeminiEmbeddingModel),
		"content": map[string]interface{}{
			"parts": parts,
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gemini api error: %s", string(body))
	}

	var result EmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Embedding.Values, nil
}

func Float32SliceToVector(v []float32) string {
	var s []string
	for _, val := range v {
		s = append(s, fmt.Sprintf("%f", val))
	}
	return "[" + strings.Join(s, ",") + "]"
}

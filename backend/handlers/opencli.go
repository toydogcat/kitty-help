package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v2"
)

type OpenCLIRequest struct {
	Args string `json:"args"`
}

// ProxyOpenCLI forwards the request to the Document Chicken worker
func ProxyOpenCLI(c *fiber.Ctx) error {
	var req OpenCLIRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	// Remote worker address
	workerURL := "http://100.103.50.4:7080/api/opencli"

	// Prepare data to send
	body, _ := json.Marshal(req)
	
	client := http.Client{Timeout: 30 * time.Second}
	resp, err := client.Post(workerURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return c.Status(502).JSON(fiber.Map{"error": "Failed to connect to Document Chicken worker", "detail": err.Error()})
	}
	defer resp.Body.Close()

	// Read response
	resBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to read worker response"})
	}

	// Return raw JSON response from worker
	var result interface{}
	if err := json.Unmarshal(resBody, &result); err != nil {
		// If not JSON, return as string
		return c.Send(resBody)
	}

	return c.Status(resp.StatusCode).JSON(result)
}

package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsPathWithinBase(t *testing.T) {
	base := filepath.Clean(filepath.Join("C:", "downloads"))

	tests := []struct {
		name   string
		target string
		want   bool
	}{
		{
			name:   "base directory itself",
			target: base,
			want:   true,
		},
		{
			name:   "file inside base directory",
			target: filepath.Join(base, "author", "video.mp4"),
			want:   true,
		},
		{
			name:   "path traversal outside base",
			target: filepath.Clean(filepath.Join(base, "..", "Windows", "system.ini")),
			want:   false,
		},
		{
			name:   "sibling directory",
			target: filepath.Clean(filepath.Join(filepath.Dir(base), "other", "video.mp4")),
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPathWithinBase(base, tt.target)
			if got != tt.want {
				t.Fatalf("isPathWithinBase(%q, %q) = %v, want %v", base, tt.target, got, tt.want)
			}
		})
	}
}

func TestHandleQueueFail_InvalidJSONReturnsBadRequest(t *testing.T) {
	handler := &ConsoleAPIHandler{}
	req := httptest.NewRequest(http.MethodPut, "/api/queue/test-id/fail", strings.NewReader("{"))
	rr := httptest.NewRecorder()

	handler.HandleQueueFail(rr, req, "test-id")

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}

	var resp APIResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Success {
		t.Fatalf("success = true, want false")
	}
	if resp.Error != "invalid request body" {
		t.Fatalf("error = %q, want %q", resp.Error, "invalid request body")
	}
}

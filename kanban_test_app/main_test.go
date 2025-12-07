package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestKanbanAppSSR(t *testing.T) {
	// Create request
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	// Call handler
	handleIndex(w, req)

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)
	html := string(body)

	// Verify content
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status OK, got %v", resp.Status)
	}

	expectedStrings := []string{
		"Kanban Test",
		"To Do",
		"In Progress",
		"Done",
		"Task 1",
		"Task 2",
		"Task 3",
		"Task 4",
		"flex h-full w-full", // Kanban board class
	}

	for _, s := range expectedStrings {
		if !strings.Contains(html, s) {
			t.Errorf("HTML does not contain %q", s)
		}
	}

    // Check if client script is included
    if !strings.Contains(html, `src="/_vango/client.js"`) {
        t.Errorf("HTML does not include client script")
    }
}

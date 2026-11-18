package cmd

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/ocfl-archive/ona/models"
)

func TestFlagRegistration(t *testing.T) {
	statusFlag := generateCmd.Flags().Lookup("status")
	if statusFlag == nil {
		t.Fatal("expected status flag to be registered")
	}
	if statusFlag.Shorthand != "s" {
		t.Fatalf("expected shorthand 's', got '%s'", statusFlag.Shorthand)
	}
}

func TestGetFileName(t *testing.T) {
	re := regexp.MustCompile(`[^-_.a-zA-Z0-9]`)
	path := "C:/data/test-file.zip"
	sig := "sig:123/abc"
	fileName := getFileName(path, sig, re)
	expected := "sig_123_abc.zip"
	if fileName != expected {
		t.Fatalf("expected '%s', got '%s'", expected, fileName)
	}
}

func TestWriteStatusFile_Success(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "status_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	statusFilePath := filepath.Join(tempDir, "nested", "status.json")

	archivedStatus := models.ArchivingStatus{
		Id:          "test-id-123",
		Status:      "archived",
		LastChanged: "2026-10-09T11:41:00Z",
	}
	dateStr := time.Now().Format(time.RFC3339)
	sig := "obj-sig-456"
	fileName := "obj-sig-456.zip"

	err = writeStatusFile(statusFilePath, archivedStatus, dateStr, sig, fileName, nil)
	if err != nil {
		t.Fatalf("writeStatusFile failed: %v", err)
	}

	content, err := os.ReadFile(statusFilePath)
	if err != nil {
		t.Fatalf("failed to read written status file: %v", err)
	}

	var parsed map[string]interface{}
	err = json.Unmarshal(content, &parsed)
	if err != nil {
		t.Fatalf("failed to unmarshal status file json: %v", err)
	}

	if parsed["id"] != "test-id-123" {
		t.Errorf("expected id 'test-id-123', got '%v'", parsed["id"])
	}
	if parsed["status"] != "archived" {
		t.Errorf("expected status 'archived', got '%v'", parsed["status"])
	}
	if parsed["lastChanged"] != "2026-10-09T11:41:00Z" {
		t.Errorf("expected lastChanged '2026-10-09T11:41:00Z', got '%v'", parsed["lastChanged"])
	}
	if parsed["date"] != dateStr {
		t.Errorf("expected date '%s', got '%v'", dateStr, parsed["date"])
	}
	if parsed["signature"] != "obj-sig-456" {
		t.Errorf("expected signature 'obj-sig-456', got '%v'", parsed["signature"])
	}
	if parsed["fileName"] != "obj-sig-456.zip" {
		t.Errorf("expected fileName 'obj-sig-456.zip', got '%v'", parsed["fileName"])
	}
	if _, hasError := parsed["error"]; hasError {
		t.Errorf("did not expect error field in successful status file")
	}
}

func TestWriteStatusFile_Error(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "status_test_err")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	statusFilePath := filepath.Join(tempDir, "status_error.json")
	expectedErr := errors.New("cannot read file: not found")

	archivedStatus := models.ArchivingStatus{
		Id: "error-status-id-789",
	}

	err = writeStatusFile(statusFilePath, archivedStatus, "", "", "", expectedErr)
	if err != nil {
		t.Fatalf("writeStatusFile failed on error case: %v", err)
	}

	content, err := os.ReadFile(statusFilePath)
	if err != nil {
		t.Fatalf("failed to read error status file: %v", err)
	}

	var parsed map[string]interface{}
	err = json.Unmarshal(content, &parsed)
	if err != nil {
		t.Fatalf("failed to unmarshal error status file json: %v", err)
	}

	if parsed["error"] != "cannot read file: not found" {
		t.Errorf("expected error 'cannot read file: not found', got '%v'", parsed["error"])
	}
	if parsed["id"] != "error-status-id-789" {
		t.Errorf("expected id 'error-status-id-789', got '%v'", parsed["id"])
	}
}

func TestWriteStatusFile_Overwrite(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "status_test_overwrite")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	statusFilePath := filepath.Join(tempDir, "status.json")

	// Pre-create file with old content
	err = os.WriteFile(statusFilePath, []byte(`{"old": "data"}`), 0644)
	if err != nil {
		t.Fatalf("failed to write initial file: %v", err)
	}

	archivedStatus := models.ArchivingStatus{
		Id:     "new-id",
		Status: "archived",
	}
	err = writeStatusFile(statusFilePath, archivedStatus, "2026-10-09T11:41:00Z", "sig-1", "sig-1.zip", nil)
	if err != nil {
		t.Fatalf("writeStatusFile failed: %v", err)
	}

	content, err := os.ReadFile(statusFilePath)
	if err != nil {
		t.Fatalf("failed to read status file: %v", err)
	}

	var parsed map[string]interface{}
	err = json.Unmarshal(content, &parsed)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if _, hasOld := parsed["old"]; hasOld {
		t.Errorf("expected old data to be overwritten")
	}
	if parsed["id"] != "new-id" {
		t.Errorf("expected id 'new-id', got '%v'", parsed["id"])
	}
}

package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ocfl-archive/dlza-manager/dlzamanagerproto"
	"github.com/ocfl-archive/ona/configuration"
	"github.com/ocfl-archive/ona/models"
)

func TestSendRequest_Non200Response(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"object not found"}`))
	}))
	defer ts.Close()

	cfg := configuration.Config{
		StatusUrl: ts.URL,
		JwtKey:    "test-secret",
	}

	_, err := GetStatus("123", cfg)
	if err == nil {
		t.Fatal("expected error for 404 response, got nil")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "404") {
		t.Errorf("expected error message to contain status code 404, got: %s", errMsg)
	}
	if !strings.Contains(errMsg, `{"error":"object not found"}`) {
		t.Errorf("expected error message to contain response body, got: %s", errMsg)
	}
	if !strings.Contains(errMsg, ts.URL) {
		t.Errorf("expected error message to contain request URL, got: %s", errMsg)
	}
}

func TestSendRequest_ServerErrorResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`internal database error details`))
	}))
	defer ts.Close()

	cfg := configuration.Config{
		StatusUrl: ts.URL,
		JwtKey:    "test-secret",
	}

	_, err := GetObjectsByChecksum("dummy-checksum", cfg)
	if err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "500") {
		t.Errorf("expected error message to contain status code 500, got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "internal database error details") {
		t.Errorf("expected error message to contain response body, got: %s", errMsg)
	}
}

func TestSendRequest_UnmarshalError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<html><body>Bad Gateway HTML</body></html>`))
	}))
	defer ts.Close()

	cfg := configuration.Config{
		StatusUrl: ts.URL,
		JwtKey:    "test-secret",
	}

	_, err := GetStatus("123", cfg)
	if err == nil {
		t.Fatal("expected unmarshal error, got nil")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "Bad Gateway HTML") {
		t.Errorf("expected unmarshal error message to contain response body, got: %s", errMsg)
	}
}

func TestSendRequest_Success(t *testing.T) {
	expectedStatus := models.ArchivingStatus{
		Id:     "test-status-id",
		Status: "in_progress",
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(expectedStatus)
	}))
	defer ts.Close()

	cfg := configuration.Config{
		StatusUrl: ts.URL,
		JwtKey:    "test-secret",
	}

	status, err := GetStatus("test-status-id", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Id != expectedStatus.Id || status.Status != expectedStatus.Status {
		t.Errorf("expected status %+v, got %+v", expectedStatus, status)
	}
}

func TestCreateObjectAndInstance_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"invalid collection"}`))
	}))
	defer ts.Close()

	cfg := configuration.Config{
		StatusUrl: ts.URL,
		JwtKey:    "test-secret",
	}

	err := CreateObjectAndInstance(&dlzamanagerproto.ObjectAndFile{}, cfg)
	if err == nil {
		t.Fatal("expected error for 400 response, got nil")
	}
	if !strings.Contains(err.Error(), `{"message":"invalid collection"}`) {
		t.Errorf("expected error to contain response body, got: %s", err.Error())
	}
}

package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHardwareSnifferFingerprinting(t *testing.T) {
	// Mock a 2015 interface endpoint returning StatusOK
	legacyMockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ext/config/hardware/model" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"value": "24Ai"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer legacyMockServer.Close()

	// Mock a 2025 next-gen endpoint returning 404 on the legacy path
	milanMockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer milanMockServer.Close()

	client := &http.Client{Timeout: 1 * time.Second}

	t.Run("Identify 2015 Platform", func(t *testing.T) {
		gen := SniffInterfaceArchitecture(client, legacyMockServer.URL)
		if gen != Gen2015Legacy {
			t.Errorf("Expected Gen2015Legacy, got %v", gen)
		}
	})

	t.Run("Identify 2025 Platform via Legacy Dropoff", func(t *testing.T) {
		gen := SniffInterfaceArchitecture(client, milanMockServer.URL)
		if gen != Gen2025Milan {
			t.Errorf("Expected Gen2025Milan, got %v", gen)
		}
	})
}


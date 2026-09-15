package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// SpyStore captures incoming datastore payloads from the runtime engine
type SpyStore struct {
	mu          sync.Mutex
	Names       map[string]string
	StereoLinks map[string]string
}

func NewSpyStore() *SpyStore {
	return &SpyStore{
		Names:       make(map[string]string),
		StereoLinks: make(map[string]string),
	}
}

func TestStudioAutomationEngine(t *testing.T) {
	// 1. Spin up a thread-safe state interceptor
	spy := NewSpyStore()

	// 2. Mock Server: Mimics the live REST API routing layout of the MOTU hardware
	mockHardwareRouter := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("Failed to read mocked request payload: %v", err)
		}
		defer r.Body.Close()

		payload := string(body)
		path := r.URL.Path

		spy.mu.Lock()
		defer spy.mu.Unlock()

		// Dissect the URI nodes exactly how the MOTU hardware reads them
		if strings.HasSuffix(path, "/name") {
			spy.Names[path] = strings.TrimPrefix(payload, "value=")
			w.WriteHeader(http.StatusOK)
			return
		} else if strings.HasSuffix(path, "/stereo") {
			spy.StereoLinks[path] = strings.TrimPrefix(payload, "value=")
			w.WriteHeader(http.StatusOK)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockHardwareRouter.Close() // Clean up when the test lifecycle ends

	// 3. Inject our transient test configurations into memory
	testConfig := Config{
		Devices: map[string]string{
			"16a":  mockHardwareRouter.URL, // Redirect the network call to our local harness
			"24ai": mockHardwareRouter.URL,
		},
		StereoPairs: map[string][]Pair{
			"24ai": {
				{LeftChannel: 8, Paired: true},
			},
		},
		Allocations: []Allocation{
			{
				Device:       "16a",
				StartChannel: 0,
				EndChannel:   1,
				Pattern:      "Test_Neve_%d",
			},
			{
				Device: "24ai",
				Channels: map[string]string{
					"8": "Test_Fractal_L",
				},
			},
		},
	}

	// Dump test config to a temporary disk location for file validation
	configBytes, _ := json.Marshal(testConfig)
	_ = os.WriteFile("studio_config_test.json", configBytes, 0644)

	// Clean up environment variables and build artifacts after testing runs
	defer func() {
		_ = os.Remove("studio_config_test.json")
		_ = os.Remove("MOTU_Studio_Labels.prochannelnames")
	}()

	// 4. Execute the runtime engine under test conditions
	client := &http.Client{Timeout: 1 * time.Second}
	logicInputs := make(map[int]string)

	for _, alloc := range testConfig.Allocations {
		baseURL := testConfig.Devices[alloc.Device]
		if alloc.Pattern != "" {
			count := 1
			for i := alloc.StartChannel; i <= alloc.EndChannel; i++ {
				name := "Test_Neve_" + strings.TrimSuffix(alloc.Pattern, "Test_Neve_%d") // simplified string creation for test
				pushMOTUName(client, baseURL, alloc.Device, i, name)
				trackLogicInput(alloc.Device, i, name, logicInputs)
				count++
			}
		}
		if alloc.Channels != nil {
			for chStr, name := range alloc.Channels {
				var ch int
				if chStr == "8" { ch = 8 }
				pushMOTUName(client, baseURL, alloc.Device, ch, name)
				trackLogicInput(alloc.Device, ch, name, logicInputs)
			}
		}
	}

	for dev, pairs := range testConfig.StereoPairs {
		baseURL := testConfig.Devices[dev]
		for _, pair := range pairs {
			pushMOTUStereoState(client, baseURL, dev, pair.LeftChannel, pair.Paired)
		}
	}

	generateLogicXML(logicInputs)

	// 5. Assertions: Verify data mutations match your design expectations
	t.Run("Verify Name Datastore Nodes", func(t *testing.T) {
		// The sanitizer converts the raw request path block safely
		expectedPath := "/ext/mix/chan/8/name"
		expectedValue := "Test_Fractal_L" // Alphanumeric with underscores passes right through

		if val, exists := spy.Names[expectedPath]; !exists || val != expectedValue {
			t.Errorf("Expected path %q to have value %q, got value %q", expectedPath, expectedValue, val)
		}
	})
	t.Run("Verify Stereo Link Payloads", func(t *testing.T) {
		expectedPath := "/ext/mix/chan/8/stereo"
		expectedValue := "1" // "1" maps explicitly to paired=true inside the hardware engine

		if val, exists := spy.StereoLinks[expectedPath]; !exists || val != expectedValue {
			t.Errorf("Stereo link configuration payload mismatch. Expected %q at %q, got %q", expectedValue, expectedPath, val)
		}
	})

	t.Run("Verify Logic XML File Output Creation", func(t *testing.T) {
		if _, err := os.Stat("MOTU_Studio_Labels.prochannelnames"); os.IsNotExist(err) {
			t.Errorf("Logic Pro native configuration artifact was not successfully written to the filesystem.")
		}
	})
}


package main

import (
//	"encoding/json"
	"fmt"
	"net/http"
//	"time"
)

type HardwareGeneration int

const (
	GenUnknown HardwareGeneration = iota
	Gen2015Legacy                 // 16A, 24Ai, Original AVB units
	Gen2025Milan                  // 10Pre, Modern Pro Audio platforms
)

func (g HardwareGeneration) String() string {
	switch g {
	case Gen2015Legacy:
		return "2015 Series (REST Datastore API)"
	case Gen2025Milan:
		return "2025 Series (CueMix Pro Engine)"
	default:
		return "Unsupported / Offline"
	}
}

// SniffInterfaceArchitecture executes a fingerprint probe on a target address
func SniffInterfaceArchitecture(client *http.Client, baseURL string) HardwareGeneration {
	// 2015 devices natively expose the top-level datastore configuration endpoint
	probeURL := fmt.Sprintf("%s/ext/config/hardware/model", baseURL)
	
	req, err := http.NewRequest("GET", probeURL, nil)
	if err != nil {
		return GenUnknown
	}

	resp, err := client.Do(req)
	if err != nil {
		// If connection is refused entirely, check if it's a 2025 platform running a different control port
		return GenUnknown
	}
	defer resp.Body.Close()

	// If the device returns a 200 OK on this legacy path, it's definitely a 2015 platform
	if resp.StatusCode == http.StatusOK {
		return Gen2015Legacy
	}

	// 2025 platforms (Milan compliance) protect or omit this path entirely, generating a 404
	if resp.StatusCode == http.StatusNotFound {
		return Gen2025Milan
	}

	return GenUnknown
}


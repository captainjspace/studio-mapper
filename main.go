package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// Config represents the master unified declarative blueprint
type Config struct {
	Devices               map[string]string     `json:"devices"`
	StereoPairs           map[string][]Pair     `json:"stereo_pairs"`
	Allocations           []Allocation          `json:"allocations"`
	StudioBusArchitecture StudioBusArchitecture `json:"studio_bus_architecture"`
}

type Pair struct {
	LeftChannel int  `json:"left_channel"`
	Paired      bool `json:"paired"`
}

type Allocation struct {
	Device       string            `json:"device"`
	StartChannel int               `json:"start_channel"`
	EndChannel   int               `json:"end_channel"`
	Pattern      string            `json:"pattern"`
	Channels     map[string]string `json:"channels"`
}

// Logic XML structures for native driver label exports
type ChannelName struct {
	InputIndex int    `xml:"index,attr"`
	CustomName string `xml:",chardata"`
}

func main() {
	configFile, err := os.Open("studio_config.json")
	if err != nil {
		fmt.Printf("❌ Failed to open configuration file: %v\n", err)
		return
	}
	defer configFile.Close()

	byteValue, _ := io.ReadAll(configFile)
	var config Config
	if err := json.Unmarshal(byteValue, &config); err != nil {
		fmt.Printf("❌ Failed to parse configuration JSON: %v\n", err)
		return
	}

	// Replace your existing name allocation loop inside main() with this adaptive generation logic:

	client := &http.Client{Timeout: 2 * time.Second}
	logicInputs := make(map[int]string)

	fmt.Println("🔍 Scanning studio network endpoints for dynamic hardware profiling...")

	for _, alloc := range config.Allocations {
		baseURL, exists := config.Devices[alloc.Device]
		if !exists {
			continue
		}

		// Dynamic Sniffing Probe Executed Here
		gen := SniffInterfaceArchitecture(client, baseURL)
		fmt.Printf("📡 Device [%s] at %s identified as: %s\n", strings.ToUpper(alloc.Device), baseURL, gen)

		switch gen {
		case Gen2015Legacy:
			// Execute the custom naming algorithms we wrote for the 2015 Datastore REST tree
			if alloc.Pattern != "" {
				count := 1
				for i := alloc.StartChannel; i <= alloc.EndChannel; i++ {
					name := fmt.Sprintf(alloc.Pattern, count)
					pushMOTUName(client, baseURL, alloc.Device, i, name)
					trackLogicInput(alloc.Device, i, name, logicInputs)
					count++
				}
			}
			if alloc.Channels != nil {
				for chStr, name := range alloc.Channels {
					ch, _ := strconv.Atoi(chStr)
					pushMOTUName(client, baseURL, alloc.Device, ch, name)
					trackLogicInput(alloc.Device, ch, name, logicInputs)
				}
			}

		case Gen2025Milan:
			// 2025 devices process routing and internal naming via CueMix Pro metadata mapping.
			// Instead of writing to their internal hardware registers (which are unneeded here since they stay open),
			// we skip the network payload overhead entirely but STILL register them to our Logic track template!
			fmt.Printf("ℹ️ Skipping hardware flash mutation for %s (Maintained as transparent digital network passthrough)\n", alloc.Device)

			if alloc.Channels != nil {
				for chStr, name := range alloc.Channels {
					ch, _ := strconv.Atoi(chStr)
					trackLogicInput(alloc.Device, ch, name, logicInputs)
				}
			}


		case GenUnknown:
			fmt.Printf("❌ Critical Network Failure: Target device [%s] did not respond to fingerprint checks.\n", alloc.Device)
			fmt.Println("🛑 Aborting installation sequence. Ensure hardware is online and dialable.")
			os.Exit(1) // Force the compiled binary to exit with a non-zero system error code
		}
	}

	// 2. Process Hardware Stereo Pairing Configurations
	for dev, pairs := range config.StereoPairs {
		baseURL, exists := config.Devices[dev]
		if !exists {
			continue
		}
		for _, pair := range pairs {
			pushMOTUStereoState(client, baseURL, dev, pair.LeftChannel, pair.Paired)
		}
	}

	// 3. Generate Logic Pro Native I/O XML Data
	generateLogicXML(logicInputs)
	// Read the active configuration payload to print your visual mix architecture
	rawBytes, err := os.ReadFile("studio_config.json")
	if err == nil {
		GenerateConsoleBlueprint(rawBytes)
	}

	fmt.Println("\n🏁 Automation engine sequence successfully executed.")
}

func pushMOTUName(client *http.Client, baseURL string, device string, ch int, name string) {
	// sanitize name
	cleanName := SanitizeHardwareName(name)

	// Path mapping explicitly interacts with MOTU's local datastore node tree
	url := fmt.Sprintf("%s/ext/mix/chan/%d/name", baseURL, ch)
	payload := []byte(fmt.Sprintf("value=%s", cleanName))

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("⚠️ Network communication timeout: %s Channel %d\n", device, ch)
		return
	}
	defer resp.Body.Close()
	fmt.Printf("✅ Mapped [%s] Input %02d ➡️ \"%s\"\n", strings.ToUpper(device), ch+1, cleanName)
}

func pushMOTUStereoState(client *http.Client, baseURL string, device string, leftCh int, paired bool) {
	url := fmt.Sprintf("%s/ext/mix/chan/%d/stereo", baseURL, leftCh)
	val := "0"
	if paired {
		val = "1"
	}
	payload := []byte(fmt.Sprintf("value=%s", val))

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	fmt.Printf("🔗 Set Stereo Link: [%s] Channels %d-%d State ➡️ %t\n", strings.ToUpper(device), leftCh+1, leftCh+2, paired)
}

func trackLogicInput(device string, ch int, name string, matrix map[int]string) {
	// Reconstructs your fixed 40-Channel network map layout into Logic tracking slices
	offset := 0
	switch device {
	case "16a":
		offset = 1 // 16A occupies DAW 1-16
	case "24ai":
		offset = 17 // 24Ai occupies DAW 17-40
	default:
		return
	}
	dawChannel := offset + ch
	matrix[dawChannel] = name
}

func generateLogicXML(matrix map[int]string) {
	var sb strings.Builder
	sb.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	sb.WriteString("<ProChannelNames>\n")

	for i := 1; i <= 48; i++ {
		name, customized := matrix[i]
		if !customized {
			// Reserve the unburdened 10Pre local hardware preamps at the tail end
			if i >= 41 {
				name = fmt.Sprintf("10Pre_Local_Pre_%d", i-40)
			} else {
				continue
			}
		}
		sb.WriteString(fmt.Sprintf("  <ChannelName index=\"%d\">%s</ChannelName>\n", i, name))
	}
	sb.WriteString("</ProChannelNames>\n")

	err := os.WriteFile("MOTU_Studio_Labels.prochannelnames", []byte(sb.String()), 0644)
	if err != nil {
		fmt.Printf("❌ Failed to compile Logic XML export file: %v\n", err)
		return
	}
	fmt.Println("\n💾 Compiled Apple Logic Pro metadata file: 'MOTU_Studio_Labels.prochannelnames'")
}

// SanitizeHardwareName cleans strings to prevent MOTU hardware internal errors
func SanitizeHardwareName(rawName string) string {
	// 1. Replace spaces with underscores
	spaced := strings.ReplaceAll(rawName, " ", "_")

	// 2. Filter out non-alphanumeric/non-standard characters natively
	sanitized := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' {
			return r
		}
		return -1 // Drops the character completely
	}, spaced)

	// 3. Enforce legacy hardware string length limit (31 chars max)
	if len(sanitized) > 31 {
		return sanitized[:31]
	}
	return sanitized
}


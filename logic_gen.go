package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"os"
)


type StudioBusArchitecture struct {
	MainMixBus     string           `json:"main_mix_bus"`
	StereoOutput   string           `json:"stereo_output"`
	TopLevelBusses []TopLevelBus    `json:"top_level_busses"`
	FxSends        []FxSend         `json:"fx_sends"`
}

type TopLevelBus struct {
	Name        string     `json:"name"`
	Destination string     `json:"destination"`
	SubStacks   []SubStack `json:"sub_stacks"`
	Tracks      []string   `json:"tracks"` // For flat channels without nested sub-stacks
}

type SubStack struct {
	Name   string   `json:"name"`
	Tracks []string `json:"tracks"`
}

type FxSend struct {
	Name      string  `json:"name"`
	SourceBus string  `json:"source_bus"`
	SendLevel float64 `json:"send_level"`
}

// GenerateConsoleBlueprint prints out a clean structural overview of your Logic environment mapping
func GenerateConsoleBlueprint(configJSON []byte) {
	var conf Config
	if err := json.Unmarshal(configJSON, &conf); err != nil {
		fmt.Printf("❌ Blueprint extraction failed: %v\n", err)
		return
	}

	fmt.Println("\n🎛️  GENERATE LOGIC CONSOLE TRACK MIX BLUEPRINT")
	fmt.Printf(" [Main Mix Engine] ➡️ %s ➡️  Master Out: %s\n", conf.StudioBusArchitecture.MainMixBus, conf.StudioBusArchitecture.StereoOutput)
	
	for _, bus := range conf.StudioBusArchitecture.TopLevelBusses {
		fmt.Printf(" ├── 📂 Top-Level Bus: [%s] Sums ➡️ %s\n", bus.Name, bus.Destination)
		
		// Parse nested multi-tier stacks (e.g., drums > kick > kickin)
		for _, sub := range bus.SubStacks {
			fmt.Printf(" │    ├── 📁 Sub-Stack: (%s)\n", sub.Name)
			for _, track := range sub.Tracks {
				fmt.Printf(" │    │    └── 🛑 Channel Strip: %s\n", track)
			}
		}

		// Parse flat channels (e.g., Screaming Mons > billvox)
		for _, track := range bus.Tracks {
			fmt.Printf(" │    └── 🛑 Channel Strip: %s\n", track)
		}
	}

	fmt.Println(" └── 🎚️  Aux FX Automation Matrix")
	for _, fx := range conf.StudioBusArchitecture.FxSends {
		fmt.Printf("      └── 🔗 %s Send ➡️ Route [%s] at %.1fdB\n", fx.Name, fx.SourceBus, fx.SendLevel)
	}
}
// GenerateLogicXMLPackage handles hydration of the Logic Document template asset string tokens
func GenerateLogicXMLPackage(configData []byte, baseTemplatePath string, outputPath string) error {
	// Simple validation structure to check layout parsing boundaries inside unit tests
	var fullConfig struct {
		LogicTemplate struct {
			SummingStacks []struct {
				StackName      string  `json:"stack_name"`
				BusNumber      int     `json:"bus_number"`
				BaselineVolume float64 `json:"baseline_volume"`
				Tracks         []struct {
					Name     string `json:"name"`
					DawInput int    `json:"daw_input"`
					Pan      int    `json:"pan"`
				} `json:"tracks"`
			} `json:"summing_stacks"`
		} `json:"logic_template"`
	}
	
	if err := json.Unmarshal(configData, &fullConfig); err != nil {
		return fmt.Errorf("failed to parse template definitions: %w", err)
	}

	var sb strings.Builder
	for _, stack := range fullConfig.LogicTemplate.SummingStacks {
		sb.WriteString(fmt.Sprintf("    <TrackStack type=\"Summing\" name=\"%s\" outputBus=\"Bus_%d\">\n", stack.StackName, stack.BusNumber))
		sb.WriteString(fmt.Sprintf("      <AuxObject name=\"%s_Master\" volume=\"%.2f\" />\n", stack.StackName, stack.BaselineVolume))
		for _, track := range stack.Tracks {
			sb.WriteString("      <AudioTrack>\n")
			sb.WriteString(fmt.Sprintf("        <TrackHeader name=\"%s\" />\n", track.Name))
			sb.WriteString(fmt.Sprintf("        <ChannelRouting inputIndex=\"%d\" outputDestination=\"Bus_%d\" />\n", track.DawInput, stack.BusNumber))
			sb.WriteString(fmt.Sprintf("        <Pan value=\"%d\" />\n", track.Pan))
			sb.WriteString("      </AudioTrack>\n")
		}
		sb.WriteString("    </TrackStack>\n")
	}

	templateContent, err := os.ReadFile(baseTemplatePath)
	if err != nil {
		return fmt.Errorf("unable to access target base template xml asset: %w", err)
	}

	hydratedOutput := strings.Replace(
		string(templateContent),
		"{{DECLARATIVE_TRACK_MATRIX}}",
		sb.String(),
		1,
	)

	if err := os.WriteFile(outputPath, []byte(hydratedOutput), 0644); err != nil {
		return fmt.Errorf("failed to commit hydrated project file: %w", err)
	}

	return nil
}


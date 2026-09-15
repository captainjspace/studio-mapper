package main

import (
	"os"
	"strings"
	"testing"
)

func TestDeclarativeTemplateHydration(t *testing.T) {
	mockJSON := `{
		"logic_template": {
			"summing_stacks": [
				{
					"stack_name": "Test_Guitar_Wall",
					"bus_number": 12,
					"baseline_volume": -4.5,
					"tracks": [
						{"name": "Test_Track_L", "daw_input": 25, "pan": -100}
					]
				}
			]
		}
	}`

	// Create a dummy skeleton base document mapping frame
	mockTemplateStructure := `<?xml version="1.0"?><LogicDocument><TrackLayout>{{DECLARATIVE_TRACK_MATRIX}}</TrackLayout></LogicDocument>`
	
	_ = os.WriteFile("test_template_base.xml", []byte(mockTemplateStructure), 0644)
	defer func() {
		_ = os.Remove("test_template_base.xml")
		_ = os.Remove("test_output_document.xml")
	}()

	// Execute structural component extraction logic
	err := GenerateLogicXMLPackage([]byte(mockJSON), "test_template_base.xml", "test_output_document.xml")
	if err != nil {
		t.Fatalf("Hydration engine crashed during execution sequence: %v", err)
	}

	// Assertions verifying structural changes match targets
	compiledContent, err := os.ReadFile("test_output_document.xml")
	if err != nil {
		t.Fatalf("Failed to retrieve generated testing file asset: %v", err)
	}

	outputStr := string(compiledContent)

	if !strings.Contains(outputStr, `name="Test_Guitar_Wall"`) {
		t.Errorf("Template processing error: Target stacking identifiers missing from final metadata.")
	}

	if !strings.Contains(outputStr, `inputIndex="25"`) {
		t.Errorf("Routing mapping failure: Track index offsets failed to match target input boundaries.")
	}

	if strings.Contains(outputStr, "{{DECLARATIVE_TRACK_MATRIX}}") {
		t.Errorf("Hydration processing failure: Structural replacement marker was not removed.")
	}
}


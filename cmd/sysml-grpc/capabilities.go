package main

import (
	"os"
	"strings"
)

const testWithholdCapabilitiesEnv = "OPENSYSML_TEST_WITHHOLD_CAPABILITIES"

func unavailableCapabilitiesForTesting() []string {
	var capabilities []string
	for _, capability := range strings.Split(os.Getenv(testWithholdCapabilitiesEnv), ",") {
		if capability = strings.TrimSpace(capability); capability != "" {
			capabilities = append(capabilities, capability)
		}
	}
	return capabilities
}

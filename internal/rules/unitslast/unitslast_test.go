package unitslast

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestReportsLeadingUnitsAndQualifiers(t *testing.T) {
	for _, tc := range []struct {
		fixture string
		want    string
	}{
		{"catch",
			"3:5: unitslast: move max to the end of maxLatencyMs\n" +
				"4:5: unitslast: move ms to the end of msTimeout\n" +
				"5:5: unitslast: move max to the end of MaxHTTPRetries\n" +
				"6:5: unitslast: move ms to the end of MSTimeout\n" +
				"7:5: unitslast: move min to the end of min_latency_ms\n" +
				"8:5: unitslast: move bytes to the end of BytesSent\n" +
				"11:2: unitslast: move ns to the end of nsLatency\n" +
				"14:11: unitslast: move us to the end of usDelay\n" +
				"15:2: unitslast: move ms to the end of msWait\n" +
				"19:5: unitslast: move kib to the end of KiB_read\n" +
				"20:5: unitslast: move mib to the end of MiBReceived\n" +
				"21:5: unitslast: move gib to the end of GiBHTTPResponse\n"},
		{"leave", ""},
	} {
		t.Run(tc.fixture, func(t *testing.T) {
			cmd := exec.CommandContext(t.Context(), "go", "run", "./cmd/anti-slop", "unitslast",
				"./internal/rules/unitslast/testdata/src/"+tc.fixture+"/...")
			cmd.Dir = "../../.."
			got, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("unitslast %s = %v, want exit 0\n%s", tc.fixture, err, got)
			}
			path := filepath.Join("internal", "rules", "unitslast", "testdata", "src", tc.fixture, tc.fixture+".go")
			var want strings.Builder
			for line := range strings.Lines(tc.want) {
				want.WriteString(path + ":" + line)
			}
			if string(got) != want.String() {
				t.Errorf("unitslast %s output:\n%s\nwant:\n%s", tc.fixture, got, want.String())
			}
		})
	}
}

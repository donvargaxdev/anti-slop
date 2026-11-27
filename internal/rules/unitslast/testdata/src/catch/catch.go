package catch

var maxLatencyMs int
var msTimeout int
var MaxHTTPRetries int
var MSTimeout int
var min_latency_ms int
var BytesSent int

type stats struct {
	nsLatency int
}

func read(usDelay int) int {
	msWait := usDelay + msTimeout
	return maxLatencyMs + msWait
}

var KiB_read int
var MiBReceived int
var GiBHTTPResponse int

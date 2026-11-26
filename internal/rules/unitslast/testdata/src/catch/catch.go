package catch

var maxLatencyMs int   // want "move max to the end of maxLatencyMs"
var msTimeout int      // want "move ms to the end of msTimeout"
var MaxHTTPRetries int // want "move max to the end of MaxHTTPRetries"
var MSTimeout int      // want "move ms to the end of MSTimeout"
var min_latency_ms int // want "move min to the end of min_latency_ms"
var BytesSent int      // want "move bytes to the end of BytesSent"

type stats struct {
	nsLatency int // want "move ns to the end of nsLatency"
}

func read(usDelay int) int { // want "move us to the end of usDelay"
	msWait := usDelay + msTimeout // want "move ms to the end of msWait"
	return maxLatencyMs + msWait
}

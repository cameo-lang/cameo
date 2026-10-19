//go:build !unix

package stdlib

// readCPUTimes returns zero CPU times where there is no getrusage(2):
// Benchmark then reports the real time alone.
func readCPUTimes() cpuTimes { return cpuTimes{} }

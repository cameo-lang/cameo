//go:build unix

package stdlib

import "syscall"

// readCPUTimes reads the CPU times from getrusage(2).
func readCPUTimes() cpuTimes {
	var self, children syscall.Rusage
	_ = syscall.Getrusage(syscall.RUSAGE_SELF, &self)
	_ = syscall.Getrusage(syscall.RUSAGE_CHILDREN, &children)
	sec := func(tv syscall.Timeval) float64 { return float64(tv.Nano()) / 1e9 }
	return cpuTimes{sec(self.Utime), sec(self.Stime), sec(children.Utime), sec(children.Stime)}
}

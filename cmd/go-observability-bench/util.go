package main

import (
	"fmt"
	"strings"
	"syscall"
	"time"
)

// errStr returns "" if err is nil or err.Error() otherwise.
func errStr(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func toDuration(t syscall.Timeval) time.Duration {
	return time.Second*time.Duration(t.Sec) + time.Microsecond*time.Duration(t.Usec)
}

// expand iterates over all keys of vars and replaces ${key} with the %v val of
// the corresponding value.
func expand(s string, vars map[string]interface{}) string {
	for k, v := range vars {
		key := fmt.Sprintf("${%s}", k)
		val := fmt.Sprintf("%v", v)
		s = strings.ReplaceAll(s, key, val)
	}
	return s
}

func closeAfter(dt time.Duration) chan struct{} {
	ch := make(chan struct{})
	time.AfterFunc(dt, func() { close(ch) })
	return ch
}

func getRusage() (r Rusage, err error) {
	var raw syscall.Rusage
	if err = syscall.Getrusage(0, &raw); err != nil {
		return
	}

	r.System = toDuration(raw.Stime)
	r.User = toDuration(raw.Utime)
	r.Signals = raw.Nsignals
	r.HardFaults = raw.Majflt
	r.SoftFaults = raw.Minflt
	r.VoluntaryContextSwitches = raw.Nvcsw
	r.InvoluntaryContextSwitches = raw.Nivcsw
	r.MaxRSS = raw.Maxrss
	return
}

type Rusage struct {
	User                       time.Duration `yaml:"user"`
	System                     time.Duration `yaml:"system"`
	Signals                    int64         `yaml:"signals"` // Warning: unmaintained in linux
	MaxRSS                     int64         `yaml:"maxrss"`  // Warning: kB in darwin, b in Linux
	SoftFaults                 int64         `yaml:"soft_faults"`
	HardFaults                 int64         `yaml:"hard_faults"`
	VoluntaryContextSwitches   int64         `yaml:"voluntary_context_switches"`
	InvoluntaryContextSwitches int64         `yaml:"involuntary_context_switches"`
}

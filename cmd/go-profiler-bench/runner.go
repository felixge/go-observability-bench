package main

import (
	"fmt"
	"runtime"
	"syscall"
	"time"

	"github.com/felixge/go-profiler-bench/workload"
	"gopkg.in/yaml.v3"
)

type RunConfig struct {
	Name        string        `yaml:"name"`
	Workload    string        `yaml:"workload"`
	Duration    time.Duration `yaml:"duration"`
	CPUProfiles int           `yaml:"cpu_profiles"`
	Outdir      string        `yaml:"outdir"`
	Args        string        `yaml:"args"`
}

type Runner struct {
	RunConfig `yaml:"config"`
	RunResult `yaml:"result"`
}

func (r *Runner) Run() error {
	r.Start = time.Now()
	r.Env.GoVersion = runtime.Version()
	r.Env.GoOS = runtime.GOOS
	r.Env.GoArch = runtime.GOARCH
	r.Env.GoMaxProcs = runtime.GOMAXPROCS(0)
	r.Env.GoNumCPU = runtime.NumCPU()

	w, err := workload.New(r.Workload, []byte(r.Args))
	if err != nil {
		return err
	}
	if err := w.Setup(); err != nil {
		return err
	}

	var before syscall.Rusage
	if err := syscall.Getrusage(0, &before); err != nil {
		return err
	}

	prof := &Profiler{
		CPUProfiles: r.CPUProfiles,
		Duration:    r.RunConfig.Duration,
		Outdir:      r.Outdir,
	}
	prof.Start()

	durationOver := closeAfter(r.RunConfig.Duration)

workloop:
	for {
		start := time.Now()
		err := w.Run()
		dt := time.Since(start)

		op := RunOp{
			Start:    start,
			Duration: dt,
			Error:    errStr(err),
		}
		r.Ops = append(r.Ops, op)
		select {
		case <-durationOver:
			if profiles, done := prof.Done(); done {
				r.Profiles = profiles
				break workloop
			}
		default:
			continue
		}
	}

	r.RunResult.Duration = time.Since(r.Start)
	var after syscall.Rusage
	if err := syscall.Getrusage(0, &after); err != nil {
		return err
	}
	r.System = toDuration(after.Stime) - toDuration(before.Stime)
	r.User = toDuration(after.Utime) - toDuration(before.Utime)

	data, err := yaml.Marshal(r)
	if err != nil {
		return err
	}
	fmt.Printf("data:\n%s\n", data)
	return nil
}

type RunResult struct {
	Start    time.Time     `yaml:"start"`
	Env      WorkloadEnv   `yaml:"env"`
	Duration time.Duration `yaml:"duration"`
	Profiles []RunProfile  `yaml:"profiles"`
	User     time.Duration `yaml:"user"`
	System   time.Duration `yaml:"system"`
	Ops      []RunOp       `yaml:"runs"`
}

type WorkloadEnv struct {
	GoVersion  string `yaml:"go_version"`
	GoOS       string `yaml:"go_os"`
	GoArch     string `yaml:"go_arch"`
	GoMaxProcs int    `yaml:"go_max_procs"`
	GoNumCPU   int    `yaml:"go_num_cpu"`
	// TODO: add kernel version
}

type RunProfile struct {
	Kind            string        `yaml:"kind"`
	File            string        `yaml:"file,omitempty"`
	Start           time.Time     `yaml:"start"`
	ProfileDuration time.Duration `yaml:"profile_duration,omitempty"`
	StopDuration    time.Duration `yaml:"stop_duration,omitempty"`
	Error           string        `yaml:"error,omitempty"`
}

type RunOp struct {
	Start    time.Time     `yaml:"start"`
	Duration time.Duration `yaml:"duration"`
	Error    string        `yaml:"error,omitempty"`
}

package main

import (
	"errors"
	"fmt"
	"runtime"
	"syscall"
	"time"

	"github.com/felixge/go-observability-bench/workload"
	"gopkg.in/yaml.v3"
)

type RunConfig struct {
	Name        string        `yaml:"name"`
	Workload    string        `yaml:"workload"`
	Concurrency int           `yaml:"concurrency"`
	Duration    time.Duration `yaml:"duration"`
	Profile     ProfileConfig `yaml:"profile"`
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
		ProfileConfig: r.Profile,
		Duration:      r.RunConfig.Duration,
		Outdir:        r.Outdir,
	}
	prof.Start()

	durationOver := closeAfter(r.RunConfig.Duration)

	workerDone := make(chan []RunOp)
	for i := 0; i < r.Concurrency; i++ {
		go func() {
			var ops []RunOp
			defer func() { workerDone <- ops }()

			for {
				start := time.Now()
				err := w.Run()
				dt := time.Since(start)

				op := RunOp{
					Start:    start,
					Duration: dt,
					Error:    errStr(err),
				}
				ops = append(ops, op)
				select {
				case <-durationOver:
					if _, done := prof.Done(); done {
						return
					}
				default:
					continue
				}
			}
		}()
	}

	r.RunResult.Duration = time.Since(r.Start)

	for i := 0; i < r.Concurrency; i++ {
		ops := <-workerDone
		r.Ops = append(r.Ops, ops...)
	}

	if profiles, done := prof.Done(); !done {
		return errors.New("bug: workers finished but profiler didn't")
	} else {
		r.Profiles = profiles
	}

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

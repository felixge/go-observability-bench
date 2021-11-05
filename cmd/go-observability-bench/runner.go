package main

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/felixge/go-observability-bench/internal"
	"github.com/felixge/go-observability-bench/workload"
	"gopkg.in/yaml.v3"
)

type RunConfig struct {
	Name        string                 `yaml:"name"`
	Workload    string                 `yaml:"workload"`
	Concurrency int                    `yaml:"concurrency"`
	Duration    time.Duration          `yaml:"duration"`
	Profile     internal.ProfileConfig `yaml:"profile"`
	Outdir      string                 `yaml:"outdir"`
	Args        string                 `yaml:"args"`
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

	r.BeforeRusage, err = getRusage()
	if err != nil {
		return err
	}
	runtime.ReadMemStats(&r.BeforeMemStats)

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

	var allOps []RunOp
	for i := 0; i < r.Concurrency; i++ {
		ops := <-workerDone
		allOps = append(allOps, ops...)
	}
	r.RunResult.Duration = time.Since(r.Start)

	csvFile, err := os.Create(filepath.Join(r.Outdir, "ops.csv"))
	if err != nil {
		return err
	}
	defer csvFile.Close()
	cw := csv.NewWriter(csvFile)
	cw.Write([]string{"start", "duration", "error"})
	for _, op := range allOps {
		cw.Write(op.ToRecord())
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		return err
	}

	if profiles, done := prof.Done(); !done {
		return errors.New("bug: workers finished but profiler didn't")
	} else {
		r.Profiles = profiles
	}

	r.AfterRusage, err = getRusage()
	if err != nil {
		return err
	}
	runtime.ReadMemStats(&r.AfterMemStats)

	data, err := yaml.Marshal(r)
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

type RunResult struct {
	Start          time.Time        `yaml:"start"`
	Env            WorkloadEnv      `yaml:"env"`
	Duration       time.Duration    `yaml:"duration"`
	BeforeRusage   Rusage           `yaml:"before_rusage"`
	AfterRusage    Rusage           `yaml:"after_rusage"`
	BeforeMemStats runtime.MemStats `yaml:"before_mem_stats"`
	AfterMemStats  runtime.MemStats `yaml:"after_mem_stats"`
	Profiles       []RunProfile     `yaml:"profiles"`
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

func (op RunOp) ToRecord() []string {
	return []string{
		op.Start.Format(time.RFC3339Nano),
		op.Duration.String(),
		op.Error,
	}
}

func (op *RunOp) FromRecord(row []string) error {
	start, err := time.Parse(time.RFC3339Nano, row[0])
	if err != nil {
		return err
	}
	op.Start = start

	duration, err := time.ParseDuration(row[1])
	if err != nil {
		return err
	}
	op.Duration = duration

	op.Error = row[2]
	return nil
}

func ReadOps(path string) ([]RunOp, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var ops []RunOp
	cr := csv.NewReader(file)
	for isHeader := true; ; isHeader = false {
		record, err := cr.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		} else if isHeader {
			continue
		}

		var op RunOp
		if err := op.FromRecord(record); err != nil {
			return nil, err
		}
		ops = append(ops, op)
	}
	return ops, nil
}

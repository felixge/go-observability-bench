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

type Runner struct {
	RunMeta `yaml:",inline"`
}

type RunMeta struct {
	internal.RunConfig `yaml:"config"`
	internal.RunResult `yaml:"result"`
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

	workerDone := make(chan []internal.RunOp)
	for i := 0; i < r.Concurrency; i++ {
		go func() {
			var ops []internal.RunOp
			defer func() { workerDone <- ops }()

			for {
				start := time.Now()
				err := w.Run()
				dt := time.Since(start)

				op := internal.RunOp{
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

	var allOps []internal.RunOp
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

func ReadOps(path string) ([]internal.RunOp, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var ops []internal.RunOp
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

		var op internal.RunOp
		if err := op.FromRecord(record); err != nil {
			return nil, err
		}
		ops = append(ops, op)
	}
	return ops, nil
}

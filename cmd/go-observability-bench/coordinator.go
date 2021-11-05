package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/felixge/go-observability-bench/internal"
	"gopkg.in/yaml.v3"
)

// Coordinator orchestrates the benchmarks described in a config file.
type Coordinator struct {
	// Config is the path to the yaml config file.
	Config string
	// Outdir is the path to the output directory.
	Outdir string
	// Bin is the path to go-observability-bench binary to use for spawning child
	// processes executing workloads.
	Bin string
	// Enable verbose output
	Verbose bool
}

func (c *Coordinator) Run() error {
	if err := os.RemoveAll(c.Outdir); err != nil {
		return err
	}

	config, err := internal.ReadConfig(c.Config)
	if err != nil {
		return err
	}

	runs, err := c.runConfigs(config)
	if err != nil {
		return err
	}

	var maxNameLength int
	var totalDuration time.Duration
	for _, run := range runs {
		if len(run.Name) > maxNameLength {
			maxNameLength = len(run.Name)
		}
		totalDuration += run.Duration
	}

	fmt.Printf("starting %d runs, expected duration: %s\n\n", len(runs), totalDuration)
	for _, run := range runs {
		if err := c.run(run, maxNameLength); err != nil {
			return err
		}
	}
	return nil
}

func (c Coordinator) runConfigs(config internal.Config) ([]internal.RunConfig, error) {
	dupeNames := map[string]int{}
	var runConfigs []internal.RunConfig
	for i := 0; i < config.Repeat; i++ {
		for _, jc := range config.Jobs {
			for _, workload := range jc.Workload {
				for _, concurrency := range jc.Concurrency {
					for _, duration := range jc.Duration {
						for _, profile := range jc.Profile {
							if profile.Period == 0 {
								profile.Period = duration
							}

							for _, args := range jc.Args {
								name := expand(jc.Name, map[string]interface{}{
									"iteration":        i,
									"workload":         workload,
									"concurrency":      concurrency,
									"duration":         duration,
									"profile_period":   profile.Period,
									"profile_cpu":      profile.CPU,
									"profile_mem":      profile.Mem,
									"profile_mem_rate": profile.MemRate,
									"profilers":        strings.Join(profile.Profilers(), ","),
								})

								dupeNames[name]++
								count := dupeNames[name]
								if count > 1 {
									name = fmt.Sprintf("%s.%d", name, count)
								}

								argsData, err := yaml.Marshal(args)
								if err != nil {
									return nil, err
								}
								runConf := internal.RunConfig{
									Name:        name,
									Iteration:   i,
									Workload:    workload,
									Concurrency: concurrency,
									Duration:    duration,
									Profile:     profile,
									Args:        string(argsData),
									Outdir:      filepath.Join(c.Outdir, name),
								}
								runConfigs = append(runConfigs, runConf)
							}
						}
					}
				}
			}
		}
	}
	return runConfigs, nil
}

func (c *Coordinator) run(rc internal.RunConfig, maxNameLength int) error {
	fmt.Printf("%s %s", rc.Name, strings.Repeat(" ", maxNameLength-len(rc.Name)))

	workloadData, err := yaml.Marshal(rc)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(rc.Outdir, 0755); err != nil {
		return err
	}

	var out bytes.Buffer
	child := exec.Command(c.Bin, "_run")
	child.Stdin = bytes.NewReader(workloadData)
	child.Stdout = &out
	child.Stderr = os.Stderr

	if c.Verbose {
		fmt.Printf("\n")
		fmt.Printf(
			"%s << EOF\n%s\nEOF\n",
			strings.Join(child.Args, " "),
			workloadData,
		)
	}

	if err := child.Run(); err != nil {
		fmt.Printf("error: %s\n", err)
		return nil
	}

	metaPath := filepath.Join(rc.Outdir, "meta.yaml")
	if err := ioutil.WriteFile(metaPath, out.Bytes(), 0644); err != nil {
		return err
	}

	meta := &RunMeta{}
	if err := yaml.Unmarshal(out.Bytes(), &meta); err != nil {
		fmt.Printf("error: %s\n", err)
		return nil
	}

	ops, err := ReadOps(filepath.Join(rc.Outdir, "ops.csv"))
	if err != nil {
		fmt.Printf("error: %s\n", err)
		return nil
	}

	var errors int
	var avg time.Duration
	var firstErr string
	for _, op := range ops {
		avg += op.Duration
		if op.Error != "" {
			errors++
			if firstErr == "" {
				firstErr = fmt.Sprintf(" (%s)", op.Error)
			}
		}
	}
	if len(ops) > 0 {
		avg = avg / time.Duration(len(ops))
	}
	magnitude := time.Duration(1)
	for {
		if magnitude > avg {
			avg = avg.Truncate(magnitude / 1000)
			break
		}
		magnitude = magnitude * 10
	}

	fmt.Printf("ops=%d avg=%s errors=%d%s\n", len(ops), avg, errors, firstErr)
	return nil
}

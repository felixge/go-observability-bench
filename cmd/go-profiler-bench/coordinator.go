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

	"gopkg.in/yaml.v3"
)

// Coordinator orchestrates the benchmarks described in a config file.
type Coordinator struct {
	// Config is the path to the yaml config file.
	Config string
	// Outdir is the path to the output directory.
	Outdir string
	// Bin is the path to go-profiler-bench binary to use for spawning child
	// processes executing workloads.
	Bin string
	// Enable verbose output
	Verbose bool
}

func (c *Coordinator) Run() error {
	if err := os.RemoveAll(c.Outdir); err != nil {
		return err
	}

	config, err := ReadConfig(c.Config)
	if err != nil {
		return err
	}

	configs, err := c.runConfigs(config)
	if err != nil {
		return err
	}

	var maxNameLength int
	for _, wc := range configs {
		if len(wc.Name) > maxNameLength {
			maxNameLength = len(wc.Name)
		}
	}

	for _, wc := range configs {
		if err := c.run(wc, maxNameLength); err != nil {
			return err
		}
	}
	return nil
}

func (c Coordinator) runConfigs(config Config) ([]RunConfig, error) {
	dupeNames := map[string]int{}
	var runConfigs []RunConfig
	for i := 0; i < config.Repeat; i++ {
		for _, jc := range config.Jobs {
			for _, workload := range jc.Workload {
				for _, duration := range jc.Duration {
					for _, cpuProfiles := range jc.CPUProfiles {
						for _, args := range jc.Args {
							name := expand(jc.Name, map[string]interface{}{
								"iteration":    i,
								"workload":     workload,
								"duration":     duration,
								"cpu_profiles": cpuProfiles,
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
							runConf := RunConfig{
								Name:        name,
								Workload:    workload,
								Duration:    duration,
								CPUProfiles: cpuProfiles,
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
	return runConfigs, nil
}

func (c *Coordinator) run(rc RunConfig, maxNameLength int) error {
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

	resultPath := filepath.Join(rc.Outdir, "result.yaml")
	if err := ioutil.WriteFile(resultPath, out.Bytes(), 0644); err != nil {
		return err
	}

	runner := &Runner{}
	if err := yaml.Unmarshal(out.Bytes(), &runner); err != nil {
		fmt.Printf("error: %s\n", err)
		return nil
	}

	var avg time.Duration
	for _, op := range runner.Ops {
		avg += op.Duration
	}
	if len(runner.Ops) > 0 {
		avg = avg / time.Duration(len(runner.Ops))
	}
	avg = avg.Truncate(time.Millisecond / 10)

	fmt.Printf("ops=%d avg=%s\n", len(runner.Ops), avg)
	return nil
}

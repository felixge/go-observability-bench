package internal

import (
	"io/fs"
	"io/ioutil"
	"path/filepath"
	"runtime"
	"time"

	"gopkg.in/yaml.v3"
)

func ReadMeta(dir string, cb func(*RunMeta, string) error) error {
	return filepath.Walk(dir, func(path string, _ fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		fileName := filepath.Base(path)
		if fileName == "meta.yaml" {
			data, err := ioutil.ReadFile(path)
			if err != nil {
				return err
			}
			meta := &RunMeta{}
			if err := yaml.Unmarshal(data, &meta); err != nil {
				return err
			}
			opsPath := filepath.Join(filepath.Dir(path), "ops.csv")
			return cb(meta, opsPath)
		}
		return nil
	})
	//data, err := ioutil.ReadFile(path)
	//if err != nil {
	//return nil, err
	//}
}

type RunMeta struct {
	RunConfig `yaml:"config"`
	RunResult `yaml:"result"`
}

type RunConfig struct {
	Name        string        `yaml:"name"`
	Workload    string        `yaml:"workload"`
	Iteration   int           `yaml:"iteration"`
	Concurrency int           `yaml:"concurrency"`
	Duration    time.Duration `yaml:"duration"`
	Profile     ProfileConfig `yaml:"profile"`
	Outdir      string        `yaml:"outdir"`
	Args        string        `yaml:"args"`
}

type RunResult struct {
	Start          time.Time        `yaml:"start"`
	Env            WorkloadEnv      `yaml:"env"`
	Duration       time.Duration    `yaml:"duration"`
	Stats          Stats            `yaml:"stats"`
	Profiles       []RunProfile     `yaml:"profiles"`
	BeforeRusage   Rusage           `yaml:"before_rusage"`
	AfterRusage    Rusage           `yaml:"after_rusage"`
	BeforeMemStats runtime.MemStats `yaml:"before_mem_stats"`
	AfterMemStats  runtime.MemStats `yaml:"after_mem_stats"`
}

type Stats struct {
	OpsCount      int           `yaml:"ops_count"`
	AvgDuration   time.Duration `yaml:"avg_duration"`
	MinDuration   time.Duration `yaml:"min_duration"`
	MaxDuration   time.Duration `yaml:"max_duration"`
	TotalDuration time.Duration `yaml:"total_duration"`
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

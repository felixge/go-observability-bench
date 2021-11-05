package internal

import (
	"io/ioutil"
	"time"

	"gopkg.in/yaml.v3"
)

func ReadConfig(path string) (c Config, err error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return c, err
	}
	err = yaml.Unmarshal(data, &c)
	c.setDefaults()
	return
}

type Config struct {
	Repeat int
	Jobs   []JobConfig `yaml:"jobs"`
}

func (c *Config) setDefaults() {
	if c.Repeat == 0 {
		c.Repeat = 1
	}
	for jIdx := range c.Jobs {
		j := &c.Jobs[jIdx]
		if len(j.Concurrency) == 0 {
			j.Concurrency = append(j.Concurrency, 1)
		}

		if len(j.Profile) == 0 {
			j.Profile = append(j.Profile, ProfileConfig{})
		}

		for pIdx := range j.Profile {
			prof := &j.Profile[pIdx]
			if prof.Block && prof.BlockRate == 0 {
				prof.BlockRate = 10000
			}
			if prof.Mutex && prof.MutexRate == 0 {
				prof.MutexRate = 10
			}
		}

		// Note: profile.Period defaults to the Job's duration, see
		// Coordinator.runConfigs().
	}
}

type JobConfig struct {
	Name        string          `yaml:"name"`
	Workload    []string        `yaml:"workload"`
	Concurrency []int           `yaml:"concurrency"`
	Duration    []time.Duration `yaml:"duration"`
	Profile     []ProfileConfig `yaml:"profile"`
	Args        []yaml.Node     `yaml:"args"`
}

type ProfileConfig struct {
	Period    time.Duration `yaml:"period"`
	CPU       bool          `yaml:"cpu"`
	Mem       bool          `yaml:"mem"`
	MemRate   int           `yaml:"mem_rate"`
	Block     bool          `yaml:"block"`
	BlockRate int           `yaml:"block_rate"`
	Mutex     bool          `yaml:"mutex"`
	MutexRate int           `yaml:"mutex_rate"`
	Goroutine bool          `yaml:"goroutine"`
	Trace     bool          `yaml:"trace"`
}

func (p ProfileConfig) Profilers() []string {
	var profilers []string
	if p.CPU {
		profilers = append(profilers, "cpu")
	}
	if p.Mem {
		profilers = append(profilers, "mem")
	}
	if p.Block {
		profilers = append(profilers, "block")
	}
	if p.Mutex {
		profilers = append(profilers, "mutex")
	}
	if p.Goroutine {
		profilers = append(profilers, "goroutine")
	}
	if p.Trace {
		profilers = append(profilers, "trace")
	}
	if len(profilers) == 0 {
		profilers = append(profilers, "none")
	}
	return profilers
}

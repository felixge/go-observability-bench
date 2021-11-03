package main

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
	for i := range c.Jobs {
		j := &c.Jobs[i]
		if len(j.Profile) == 0 {
			j.Profile = append(j.Profile, ProfileConfig{})
			continue
		}

		// Note: profile.Period defaults to the Job's duration, see
		// Coordinator.runConfigs().
	}
}

type JobConfig struct {
	Name     string          `yaml:"name"`
	Workload []string        `yaml:"workload"`
	Duration []time.Duration `yaml:"duration"`
	Profile  []ProfileConfig `yaml:"profile"`
	Args     []yaml.Node     `yaml:"args"`
}

type ProfileConfig struct {
	Period  time.Duration `yaml:"period"`
	CPU     bool          `yaml:"cpu"`
	Mem     bool          `yaml:"mem"`
	MemRate int           `yaml:"mem_rate"`
	Trace   bool          `yaml:"trace"`
}

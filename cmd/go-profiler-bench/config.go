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
}

type JobConfig struct {
	Name        string          `yaml:"name"`
	Workload    []string        `yaml:"workload"`
	Duration    []time.Duration `yaml:"duration"`
	CPUProfiles []int           `yaml:"cpu_profiles"`
	Args        []yaml.Node     `yaml:"args"`
}

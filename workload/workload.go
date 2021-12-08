package workload

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

type Workload interface {
	Setup() error
	Run() error
}

func New(name string, args []byte) (Workload, error) {
	var w Workload
	switch name {
	case "json":
		w = &JSON{}
	case "http":
		w = &HTTP{}
	case "chan":
		w = &Chan{}
	case "mutex":
		w = &Mutex{}
	case "sql":
		w = &SQL{}
	default:
		return nil, fmt.Errorf("unknown workload: %q", name)
	}
	return w, yaml.Unmarshal(args, w)
}

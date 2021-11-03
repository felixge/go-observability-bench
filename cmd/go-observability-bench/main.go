package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	"gopkg.in/yaml.v3"
)

const usage = "usage: go-observability-bench <config> <outdir>"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	var (
		verboseF = flag.Bool("v", false, "Verbose output")
	)
	flag.Parse()

	var runner interface{ Run() error }
	switch arg0 := flag.Arg(0); arg0 {
	case "_run":
		data, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		r := Runner{}
		if err := yaml.Unmarshal(data, &r.RunConfig); err != nil {
			return err
		}
		runner = &r
	default:
		arg1 := flag.Arg(1)
		if arg0 == "" {
			return fmt.Errorf("error: no config (%s)", usage)
		} else if arg1 == "" {
			return fmt.Errorf("error: no outdir (%s)", usage)
		}

		runner = &Coordinator{
			Bin:     os.Args[0],
			Config:  arg0,
			Outdir:  arg1,
			Verbose: *verboseF,
		}
	}
	return runner.Run()
}

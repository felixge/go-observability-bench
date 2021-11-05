package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/felixge/go-observability-bench/internal"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	tracer.Start(
		tracer.WithEnv("ci"),
		tracer.WithService("go-observability-bench"),
		tracer.WithServiceVersion("dev"),
	)
	defer tracer.Stop()

	flag.Parse()
	var start time.Time
	var end time.Time

	var runs []*internal.RunMeta
	err := internal.ReadMeta(flag.Arg(0), func(meta *internal.RunMeta) error {
		r := meta.RunResult
		if start.IsZero() || r.Start.Before(start) {
			start = r.Start
		}
		runEnd := r.Start.Add(r.Duration)
		if end.IsZero() || runEnd.After(end) {
			end = runEnd
		}
		runs = append(runs, meta)
		return nil
	})
	if err != nil {
		return err
	}
	benchSpan := tracer.StartSpan(
		"bench",
		tracer.StartTime(start),
	)
	defer benchSpan.Finish(tracer.FinishTime(end))
	for _, run := range runs {
		r := run.RunResult
		runSpan := tracer.StartSpan(
			"run",
			tracer.ServiceName(run.Workload),
			tracer.StartTime(run.Start),
			tracer.ChildOf(benchSpan.Context()),
			tracer.Tag("concurrency", run.Concurrency),
			tracer.Tag("iteration", run.Iteration),
			tracer.Tag("name", run.Name),
			tracer.Tag("profiles", run.Profile.Profilers()),
		)
		runSpan.Finish(tracer.FinishTime(r.Start.Add(r.Duration)))
	}
	fmt.Printf("Finished\n")

	return nil
}

package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/DataDog/datadog-go/statsd"
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
	statsd, err := statsd.New("127.0.0.1:8125")
	if err != nil {
		return err
	}

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
	err = internal.ReadMeta(flag.Arg(0), func(meta *internal.RunMeta) error {
		r := meta.RunResult
		if start.IsZero() || r.Start.Before(start) {
			start = r.Start
		}
		runEnd := r.Start.Add(r.Duration)
		if end.IsZero() || runEnd.After(end) {
			end = runEnd
		}
		runs = append(runs, meta)

		tags := []string{
			fmt.Sprintf("iteration:%d", meta.Iteration),
			fmt.Sprintf("profilers:%s", strings.Join(meta.Profile.Profilers(), "&")),
			fmt.Sprintf("name:%s", meta.Name),
			fmt.Sprintf("workload:%s", meta.Workload),
			fmt.Sprintf("concurrency:%d", meta.Concurrency),
		}
		statsd.Gauge("go-observability-bench.stats.avg", r.Stats.Avg.Seconds(), tags, 1)
		statsd.Gauge("go-observability-bench.stats.ops", float64(r.Stats.Ops), tags, 1)
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
			tracer.Tag("latency.ops", run.Stats.Ops),
			tracer.Tag("latency.avg", run.Stats.Avg),
		)
		runSpan.Finish(tracer.FinishTime(r.Start.Add(r.Duration)))
	}
	if err := statsd.Close(); err != nil {
		return err
	}
	return nil
}

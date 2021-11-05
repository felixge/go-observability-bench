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
	statsd, err := statsd.New("127.0.1:8125")
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
			fmt.Sprintf("go_version:%s", meta.Env.GoVersion),
			fmt.Sprintf("go_os:%s", meta.Env.GoOS),
			fmt.Sprintf("go_arch:%s", meta.Env.GoArch),
			fmt.Sprintf("go_max_procs:%d", meta.Env.GoMaxProcs),
			fmt.Sprintf("go_num_cpu:%d", meta.Env.GoNumCPU),
		}

		userDuration := r.AfterRusage.User - r.BeforeRusage.User
		sysDuration := r.AfterRusage.System - r.BeforeRusage.System

		statsd.Gauge("go-observability-bench.stats.heap_in_use", float64(r.AfterMemStats.HeapInuse), tags, 1)
		statsd.Gauge("go-observability-bench.stats.buck_hash_sys", float64(r.AfterMemStats.BuckHashSys), tags, 1)
		statsd.Gauge("go-observability-bench.stats.user_duration", userDuration.Seconds(), tags, 1)
		statsd.Gauge("go-observability-bench.stats.system_duration", sysDuration.Seconds(), tags, 1)
		statsd.Gauge("go-observability-bench.stats.min_duration", r.Stats.MinDuration.Seconds(), tags, 1)
		statsd.Gauge("go-observability-bench.stats.avg_duration", r.Stats.AvgDuration.Seconds(), tags, 1)
		statsd.Gauge("go-observability-bench.stats.max_duration", r.Stats.MaxDuration.Seconds(), tags, 1)
		statsd.Gauge("go-observability-bench.stats.total_duration", r.Stats.TotalDuration.Seconds(), tags, 1)
		statsd.Gauge("go-observability-bench.stats.ops_count", float64(r.Stats.OpsCount), tags, 1)
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
			tracer.Tag("latency.ops", run.Stats.OpsCount),
			tracer.Tag("latency.avg", run.Stats.AvgDuration),
		)
		runSpan.Finish(tracer.FinishTime(r.Start.Add(r.Duration)))
	}
	if err := statsd.Close(); err != nil {
		return err
	}
	return nil
}

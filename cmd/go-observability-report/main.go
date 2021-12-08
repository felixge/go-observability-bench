package main

import (
	"bytes"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/DataDog/datadog-go/statsd"
	"github.com/felixge/go-observability-bench/internal"
	"github.com/iancoleman/strcase"
	"github.com/montanaflynn/stats"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	flag.Parse()
	table, err := Analyze(flag.Arg(0))
	if err != nil {
		return err
	}

	statsd, err := statsd.New("127.0.1:8125")
	if err != nil {
		return err
	}
	if err := WriteGoBench("gobench", table); err != nil {
		return err
	} else if err := SendStatsd(statsd, table); err != nil {
		return err
	}

	return statsd.Close()
}

func WriteGoBench(dir string, table []*ConfigSummary) error {
	profilers := map[string]*bytes.Buffer{}
	for _, s := range table {
		out := profilers[s.Profilers]
		if out == nil {
			out = &bytes.Buffer{}
			profilers[s.Profilers] = out
		}

		name := fmt.Sprintf("Benchmark%s_C%d", strcase.ToCamel(s.Workload), s.Concurrency)
		for _, r := range s.Runs {
			fmt.Fprintf(out, "%s  %d  %d ns/op\n", name, r.Ops, r.Mean.Nanoseconds())
		}
		fmt.Fprintf(out, "\n")
	}

	if err := os.RemoveAll(dir); err != nil {
		return err
	} else if err := os.Mkdir(dir, 0755); err != nil {
		return err
	}

	for profiler, out := range profilers {
		txtPath := filepath.Join(dir, profiler+".txt")
		if err := ioutil.WriteFile(txtPath, out.Bytes(), 0644); err != nil {
			return err
		}
	}

	return nil
}

/*
BenchmarkJSON-12             469           2561828 ns/op
BenchmarkJSON-12             468           2568088 ns/op
*/

func Analyze(dir string) ([]*ConfigSummary, error) {
	configOps := map[Config][][]*internal.RunOp{}
	err := internal.ReadMeta(dir, func(meta *internal.RunMeta, opsPath string) error {
		csvFile, err := os.Open(opsPath)
		if err != nil {
			return err
		}
		defer csvFile.Close()
		cr := csv.NewReader(csvFile)
		var ops []*internal.RunOp
		for header := true; ; header = false {
			record, err := cr.Read()
			if err == io.EOF {
				break
			} else if err != nil {
				return err
			} else if header {
				continue
			}

			var op internal.RunOp
			if err := op.FromRecord(record); err != nil {
				return err
			}
			ops = append(ops, &op)
		}
		profilers := strings.Join(meta.Profile.Profilers(), "+")
		config := Config{
			Workload:    meta.Workload,
			Concurrency: meta.Concurrency,
			Profilers:   profilers,
		}
		configOps[config] = append(configOps[config], ops)
		return nil
	})
	if err != nil {
		return nil, err
	}

	var sList []*ConfigSummary
	sMap := map[Config]*ConfigSummary{}
	for config, runs := range configOps {
		summary := &ConfigSummary{}
		var (
			allDurations []time.Duration
			runMeans     []time.Duration
			runP99s      []time.Duration
		)
		for _, runOps := range runs {
			var runDurations []time.Duration
			for _, op := range runOps {
				runDurations = append(runDurations, op.Duration)
			}
			runMean := durationMean(runDurations)
			summary.Runs = append(summary.Runs, &Run{
				Mean: runMean,
				Ops:  len(runOps),
			})
			runMeans = append(runMeans, runMean)
			runP99s = append(runP99s, durationPercentile(runDurations, 99))
			allDurations = append(allDurations, runDurations...)
		}

		summary.Ops = len(allDurations)
		summary.P99 = durationPercentile(allDurations, 99)
		summary.P99Stdev = durationStdev(runP99s)
		summary.Mean = durationMean(allDurations)
		summary.MeanStdev = durationStdev(runMeans)
		summary.Config = config
		sList = append(sList, summary)
		sMap[config] = summary
	}

	for _, s := range sList {
		noneKey := s.Config
		noneKey.Profilers = "none"
		s.MeanInc = (float64(s.Mean)/float64(sMap[noneKey].Mean) - 1) * 100
		s.P99Inc = (float64(s.P99)/float64(sMap[noneKey].P99) - 1) * 100
	}

	return sList, nil
}

type Config struct {
	Workload    string
	Concurrency int
	Profilers   string
}

type ConfigSummary struct {
	Config
	Ops       int
	Mean      time.Duration
	MeanStdev time.Duration
	MeanInc   float64

	P99      time.Duration
	P99Stdev time.Duration
	P99Inc   float64

	Runs []*Run

	// TODO more stuff, p99, p99inc, etc.
}

type Run struct {
	Mean time.Duration
	Ops  int
}

func durationStdev(durations []time.Duration) time.Duration {
	stdev, _ := stats.StdDevS(durationsToFloats(durations))
	return time.Duration(stdev)
}

func durationMean(durations []time.Duration) time.Duration {
	stdev, _ := stats.Mean(durationsToFloats(durations))
	return time.Duration(stdev)
}

func durationPercentile(durations []time.Duration, percentile float64) time.Duration {
	stdev, _ := stats.Percentile(durationsToFloats(durations), percentile)
	return time.Duration(stdev)
}

func durationsToFloats(durations []time.Duration) stats.Float64Data {
	floats := make(stats.Float64Data, len(durations))
	for i, d := range durations {
		floats[i] = float64(d)
	}
	return floats
}

func SendStatsd(statsd *statsd.Client, table []*ConfigSummary) error {
	for _, s := range table {
		if err := s.SendStatsd(statsd); err != nil {
			return err
		}
	}
	return nil
}

func (s *ConfigSummary) SendStatsd(client *statsd.Client) error {
	tags := []string{
		fmt.Sprintf("profilers:%s", s.Profilers),
		fmt.Sprintf("workload:%s", s.Workload),
		fmt.Sprintf("concurrency:%d", s.Concurrency),
	}
	client.Gauge("go11y.ops", float64(s.Ops), tags, 1)
	client.Gauge("go11y.mean", s.Mean.Seconds(), tags, 1)
	client.Gauge("go11y.mean_stdev", s.MeanStdev.Seconds(), tags, 1)
	client.Gauge("go11y.mean_inc", s.MeanInc, tags, 1)
	client.Gauge("go11y.p99", s.P99.Seconds(), tags, 1)
	client.Gauge("go11y.p99_stdev", s.P99Stdev.Seconds(), tags, 1)
	client.Gauge("go11y.p99_inc", s.P99Inc, tags, 1)
	return nil
}

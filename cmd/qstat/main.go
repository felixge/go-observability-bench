package main

import (
	"bytes"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/felixge/go-observability-bench/internal"
	"github.com/olekukonko/tablewriter"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	flag.Parse()
	txts, err := filepath.Glob(filepath.Join(flag.Arg(0), "*.txt"))
	if err != nil {
		return err
	}

	var nonePath string
	for _, benchPath := range txts {
		if profilerName(benchPath) == "none" {
			nonePath = benchPath
		}
	}
	var allRows []*bsRow
	for _, benchPath := range txts {
		profiler := profilerName(benchPath)
		if profiler == "none" {
			continue
		}
		rows, err := benchStat(nonePath, benchPath)
		if err != nil {
			return err
		}
		allRows = append(allRows, rows...)
	}

	outRows := outRows(allRows)
	order := []string{"none", "cpu", "mem", "mutex", "block", "goroutine", "trace", "all"}
	for workload, rows := range outRows {
		sort.Slice(rows, func(i, j int) bool {
			io := stringIndex(rows[i].Profiler, order)
			jo := stringIndex(rows[j].Profiler, order)
			if io == jo {
				return rows[i].Concurrency < rows[j].Concurrency
			}
			return io < jo
		})
		fmt.Printf("workload: %s\n", workload)

		tw := tablewriter.NewWriter(os.Stdout)
		tw.SetHeader([]string{"Profiler", "Concurrency", "Old Time/Op", "+/-", "New Time/Op", "+/-", "Delta", ""})
		tw.SetBorder(false)
		tw.SetCenterSeparator("")
		tw.SetColumnSeparator("")
		tw.SetRowSeparator("")
		tw.SetHeaderLine(false)
		for _, r := range rows {
			profiler := r.Profiler
			if profiler == "cpu+mem+block+mutex+goroutine+trace" {
				profiler = "all"
			}
			tw.Append([]string{
				profiler,
				r.Concurrency,
				internal.TruncateDuration(r.NoneMean).String(),
				r.NoneDev,
				internal.TruncateDuration(r.ProfilerMean).String(),
				r.ProfilerDev,
				r.Delta,
				r.PVal,
			})
		}
		tw.Render()
	}
	return nil
}

func stringIndex(s string, slice []string) int {
	for i, v := range slice {
		if v == s {
			return i
		}
	}
	return -1
}

func outRows(rows []*bsRow) map[string][]*outRow {
	out := map[string][]*outRow{}
	//noneRows := map[string]bool{}
	for _, row := range rows {
		or := &outRow{
			Profiler:     row.Profiler,
			Concurrency:  row.Concurrency,
			NoneMean:     row.None,
			NoneDev:      row.NoneDev,
			ProfilerMean: row.After,
			ProfilerDev:  row.AfterDev,
			Delta:        row.Delta,
			PVal:         row.PVal,
		}
		out[row.Workload] = append(out[row.Workload], or)

		//key := fmt.Sprintf("%s:%s", row.Workload, row.Concurrency)
		//if !noneRows[key] {
		//or := &outRow{
		//Profiler:     "none",
		//Concurrency:  row.Concurrency,
		//ProfilerMean: row.None,
		//ProfilerDev:  row.NoneDev,
		//Delta:        "",
		//PVal:         "",
		//}
		//out[row.Workload] = append(out[row.Workload], or)
		//noneRows[key] = true
		//}
	}
	return out
}

func profilerName(txtPath string) string {
	return strings.TrimSuffix(filepath.Base(txtPath), ".txt")
}

func benchStat(before, after string) ([]*bsRow, error) {
	buf := &bytes.Buffer{}
	cmd := exec.Command("benchstat", "-csv", before, after)
	cmd.Stdout = buf
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	var rows []*bsRow
	cr := csv.NewReader(buf)
	for header := true; ; header = false {
		record, err := cr.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		} else if header {
			continue
		}

		noneVal, err := strconv.ParseFloat(record[1], 64)
		if err != nil {
			return nil, err
		}
		afterVal, err := strconv.ParseFloat(record[3], 64)
		if err != nil {
			return nil, err
		}
		wc := strings.Split(record[0], "_C")

		profiler := profilerName(after)
		if profiler == "cpu+mem+block+mutex+goroutine+trace" {
			profiler = "all"
		}

		row := &bsRow{
			Profiler:    profiler,
			Workload:    wc[0],
			Concurrency: wc[1],
			None:        time.Duration(noneVal),
			NoneDev:     record[2],
			After:       time.Duration(afterVal),
			AfterDev:    record[4],
			Delta:       record[5],
			PVal:        record[6],
		}
		rows = append(rows, row)
	}
	return rows, nil
}

type bsRow struct {
	Profiler    string
	Workload    string
	Concurrency string
	None        time.Duration
	NoneDev     string
	After       time.Duration
	AfterDev    string
	Delta       string
	PVal        string
}

type outRow struct {
	Profiler     string
	Concurrency  string
	NoneMean     time.Duration
	NoneDev      string
	ProfilerMean time.Duration
	ProfilerDev  string
	Delta        string
	PVal         string
}

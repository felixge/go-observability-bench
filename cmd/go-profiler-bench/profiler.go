package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"runtime/trace"
	"time"
)

type Profiler struct {
	ProfileConfig
	Duration time.Duration
	Outdir   string

	doneCh    chan struct{}
	cpuBuf    bytes.Buffer
	memBuf    bytes.Buffer
	traceBuf  bytes.Buffer
	profiles  []RunProfile
	cpuProf   *RunProfile
	memProf   *RunProfile
	traceProf *RunProfile

	bufs  map[string]*bytes.Buffer
	profs map[string]*RunProfile
}

type profiler struct {
	Kind  string
	Init  func(*Profiler)
	Start func(*Profiler, io.Writer) (bool, error)
}

var profilers = []profiler{
	{
		Kind: "cpu",
		//Enabled: func(c ProfileConfig) bool { return c.CPU },
		Start: func(p *Profiler, w io.Writer) (bool, error) {
			if !p.CPU {
				return false, nil
			}
			return true, pprof.StartCPUProfile(w)
		},
	},
	{
		Kind: "mem",
		Init: func(p *Profiler) {
			if p.Mem && p.MemRate != 0 {
				runtime.MemProfileRate = p.MemRate
			}
		},
		Start: func(p *Profiler, _ io.Writer) (bool, error) {
			return p.Mem, nil
		},
	},
	{
		Kind: "trace",
		//Enabled: func(c ProfileConfig) bool { return c.Trace },
	},
}

func (p *Profiler) Start() {
	p.doneCh = make(chan struct{})
	p.bufs = make(map[string]*bytes.Buffer)
	p.profs = make(map[string]*RunProfile)

	if enabled := p.startProfiles(); enabled == 0 {
		close(p.doneCh)
		return
	}
	go p.profileLoop()
}

func (p *Profiler) startProfiles() int {
	var enabledCount int
	for _, prof := range profilers {
		var buf *bytes.Buffer
		if buf = p.bufs[prof.Kind]; buf == nil {
			buf = new(bytes.Buffer)
		} else {
			buf.Reset() // TODO: lowers allocs, but increases max(heap)
		}
		start := time.Now()
		enabled, err := prof.Start(p, buf)
		if enabled {
			enabledCount++
			p.profiles = append(p.profiles, RunProfile{
				Kind:  prof.Kind,
				Start: start,
				Error: errStr(err),
			})
			p.bufs[prof.Kind] = buf
			p.profs[prof.Kind] = &p.profiles[len(p.profiles)-1]
		}
	}
	return enabledCount
}

func (p *Profiler) Done() ([]RunProfile, bool) {
	select {
	case <-p.doneCh:
		return p.profiles, true
	default:
		return nil, false
	}
}

func (p *Profiler) profileLoop() {
	defer close(p.doneCh)
	loopStart := time.Now()
	tick := time.NewTicker(p.Period)
	for i := 0; ; {
		<-tick.C

		if p.CPU {
			cpuStop := time.Now()
			p.cpuProf.ProfileDuration = cpuStop.Sub(p.cpuProf.Start)
			pprof.StopCPUProfile()
			p.cpuProf.StopDuration = time.Since(cpuStop)
			p.cpuProf.File = fmt.Sprintf("cpu.%d.pprof", i)
			profPath := filepath.Join(p.Outdir, p.cpuProf.File)
			if err := ioutil.WriteFile(profPath, p.cpuBuf.Bytes(), 0644); err != nil && p.cpuProf.Error == "" {
				p.cpuProf.Error = errStr(err)
			}
		}

		if p.Mem {
			memStop := time.Now()
			p.memProf.ProfileDuration = memStop.Sub(p.memProf.Start)
			err := pprof.Lookup("allocs").WriteTo(&p.memBuf, 0)
			if err != nil && p.memProf.Error == "" {
				p.memProf.Error = errStr(err)
			}
			p.memProf.StopDuration = time.Since(memStop)
			p.memProf.File = fmt.Sprintf("mem.%d.pprof", i)
			profPath := filepath.Join(p.Outdir, p.memProf.File)
			if err := ioutil.WriteFile(profPath, p.memBuf.Bytes(), 0644); err != nil && p.memProf.Error == "" {
				p.memProf.Error = errStr(err)
			}
		}

		if p.Trace {
			traceStop := time.Now()
			p.traceProf.ProfileDuration = traceStop.Sub(p.traceProf.Start)
			trace.Stop()
			p.traceProf.StopDuration = time.Since(traceStop)
			p.traceProf.File = fmt.Sprintf("trace.%d.out", i)
			profPath := filepath.Join(p.Outdir, p.traceProf.File)
			if err := ioutil.WriteFile(profPath, p.traceBuf.Bytes(), 0644); err != nil && p.traceProf.Error == "" {
				p.traceProf.Error = errStr(err)
			}
		}

		if time.Since(loopStart) >= p.Duration {
			break
		}

		if p.CPU {
			p.startCPUProfile()
		}
		if p.Mem {
			p.startMemProfile()
		}
		if p.Trace {
			p.startTrace()
		}
	}
}

func (p *Profiler) startCPUProfile() {
	p.cpuBuf.Reset()
	startErr := pprof.StartCPUProfile(&p.cpuBuf)
	p.profiles = append(p.profiles, RunProfile{
		Kind:  "cpu",
		Start: time.Now(),
		Error: errStr(startErr),
	})
	p.cpuProf = &p.profiles[len(p.profiles)-1]
}

func (p *Profiler) startTrace() {
	p.traceBuf.Reset()
	startErr := trace.Start(&p.traceBuf)
	p.profiles = append(p.profiles, RunProfile{
		Kind:  "trace",
		Start: time.Now(),
		Error: errStr(startErr),
	})
	p.traceProf = &p.profiles[len(p.profiles)-1]
}

func (p *Profiler) startMemProfile() {
	p.memBuf.Reset()
	p.profiles = append(p.profiles, RunProfile{
		Kind:  "mem",
		Start: time.Now(),
	})
	p.memProf = &p.profiles[len(p.profiles)-1]
}

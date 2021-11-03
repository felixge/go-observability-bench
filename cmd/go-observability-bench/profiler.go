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
	"strings"
	"time"
)

type Profiler struct {
	ProfileConfig
	Duration time.Duration
	Outdir   string

	doneCh   chan struct{}
	profiles []RunProfile
	bufs     map[string]*bytes.Buffer
	profs    map[string]*RunProfile
}

type profiler struct {
	Kind    string
	Enabled func(ProfileConfig) bool
	Init    func(ProfileConfig)
	Start   func(io.Writer) error
	Stop    func(io.Writer) error
}

var profilers = []profiler{
	{
		Kind:    "cpu.pprof",
		Enabled: func(c ProfileConfig) bool { return c.CPU },
		Start: func(w io.Writer) error {
			return pprof.StartCPUProfile(w)
		},
		Stop: func(_ io.Writer) error {
			pprof.StopCPUProfile()
			return nil
		},
	},

	{
		Kind:    "mem.pprof",
		Enabled: func(c ProfileConfig) bool { return c.Mem },
		Init: func(c ProfileConfig) {
			if c.MemRate != 0 {
				runtime.MemProfileRate = c.MemRate
			}
		},
		Stop: func(w io.Writer) error {
			return pprof.Lookup("allocs").WriteTo(w, 0)
		},
	},

	{
		Kind:    "block.pprof",
		Enabled: func(c ProfileConfig) bool { return c.Block },
		Init: func(c ProfileConfig) {
			if c.BlockRate != 0 {
				runtime.SetBlockProfileRate(c.BlockRate)
			}
		},
		Stop: func(w io.Writer) error {
			return pprof.Lookup("block").WriteTo(w, 0)
		},
	},

	{
		Kind:    "mutex.pprof",
		Enabled: func(c ProfileConfig) bool { return c.Mutex },
		Init: func(c ProfileConfig) {
			if c.MutexRate != 0 {
				runtime.SetMutexProfileFraction(c.MutexRate)
			}
		},
		Stop: func(w io.Writer) error {
			return pprof.Lookup("mutex").WriteTo(w, 0)
		},
	},

	{
		Kind:    "goroutine.pprof",
		Enabled: func(c ProfileConfig) bool { return c.Mutex },
		Stop: func(w io.Writer) error {
			return pprof.Lookup("goroutine").WriteTo(w, 0)
		},
	},

	{
		Kind:    "trace.out",
		Enabled: func(c ProfileConfig) bool { return c.Trace },
		Start: func(w io.Writer) error {
			return trace.Start(w)
		},
		Stop: func(_ io.Writer) error {
			trace.Stop()
			return nil
		},
	},
}

func (p *Profiler) Start() {
	p.doneCh = make(chan struct{})
	p.bufs = make(map[string]*bytes.Buffer)
	p.profs = make(map[string]*RunProfile)

	if enabled := p.startProfiles(0); enabled == 0 {
		close(p.doneCh)
		return
	}
	go p.profileLoop()
}

func (p *Profiler) startProfiles(iteration int) int {
	var enabled int
	for _, prof := range profilers {
		if !prof.Enabled(p.ProfileConfig) {
			continue
		}
		enabled++

		if iteration == 0 && prof.Init != nil {
			prof.Init(p.ProfileConfig)
		}

		var buf *bytes.Buffer
		if buf = p.bufs[prof.Kind]; buf == nil {
			buf = new(bytes.Buffer)
		} else {
			buf.Reset() // TODO: lowers allocs, but increases max(heap)
		}
		start := time.Now()
		var startErr error
		if prof.Start != nil {
			startErr = prof.Start(buf)
		}

		p.profiles = append(p.profiles, RunProfile{
			Kind:  prof.Kind,
			Start: start,
			Error: errStr(startErr),
		})
		p.bufs[prof.Kind] = buf
		p.profs[prof.Kind] = &p.profiles[len(p.profiles)-1]
	}
	return enabled
}

func (p *Profiler) stopProfiles(iteration int) {
	for _, prof := range profilers {
		if !prof.Enabled(p.ProfileConfig) {
			continue
		}

		record := p.profs[prof.Kind]
		buf := p.bufs[prof.Kind]
		stop := time.Now()
		record.ProfileDuration = stop.Sub(record.Start)
		if prof.Stop != nil {
			if err := prof.Stop(buf); err != nil && record.Error == "" {
				record.Error = errStr(err)
			}
		}
		record.StopDuration = time.Since(stop)
		kind := strings.Split(prof.Kind, ".")
		record.File = fmt.Sprintf("%s.%d.%s", kind[0], iteration, kind[1])
		profPath := filepath.Join(p.Outdir, record.File)
		writErr := ioutil.WriteFile(profPath, buf.Bytes(), 0644)
		if writErr != nil && record.Error == "" {
			record.Error = errStr(writErr)
		}
	}
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
		p.stopProfiles(i)
		if time.Since(loopStart) >= p.Duration {
			return
		}
		p.startProfiles(i + 1)
	}
}

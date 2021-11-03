package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"runtime/pprof"
	"time"
)

type Profiler struct {
	CPUProfiles int
	Duration    time.Duration
	Outdir      string

	doneCh   chan struct{}
	cpuBuf   bytes.Buffer
	profiles []RunProfile
}

func (p *Profiler) Start() {
	p.doneCh = make(chan struct{})
	if p.CPUProfiles > 0 {
		p.startCPUProfile()
		go p.cpuProfileLoop()
	} else {
		close(p.doneCh)
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

func (p *Profiler) cpuProfileLoop() {
	defer close(p.doneCh)
	tick := time.NewTicker(p.Duration / (time.Duration(p.CPUProfiles)))
	for i := 0; ; {
		<-tick.C

		stop := time.Now()
		pprof.StopCPUProfile()
		r := &p.profiles[len(p.profiles)-1]
		r.ProfileDuration = stop.Sub(r.Start)
		r.StopDuration = time.Since(stop)
		r.File = fmt.Sprintf("cpu.%d.pprof", i)
		profPath := filepath.Join(p.Outdir, r.File)
		if err := ioutil.WriteFile(profPath, p.cpuBuf.Bytes(), 0644); err != nil && r.Error == "" {
			r.Error = errStr(err)
		}

		i++
		if i >= p.CPUProfiles {
			break
		}

		p.startCPUProfile()
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
}

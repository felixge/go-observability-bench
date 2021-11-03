package main

import (
	"fmt"
	"strings"
	"syscall"
	"time"
)

// errStr returns "" if err is nil or err.Error() otherwise.
func errStr(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func toDuration(t syscall.Timeval) time.Duration {
	return time.Second*time.Duration(t.Sec) + time.Microsecond*time.Duration(t.Usec)
}

// expand iterates over all keys of vars and replaces ${key} with the %v val of
// the corresponding value.
func expand(s string, vars map[string]interface{}) string {
	for k, v := range vars {
		key := fmt.Sprintf("${%s}", k)
		val := fmt.Sprintf("%v", v)
		s = strings.ReplaceAll(s, key, val)
	}
	return s
}

func closeAfter(dt time.Duration) chan struct{} {
	ch := make(chan struct{})
	time.AfterFunc(dt, func() { close(ch) })
	return ch
}

package workload

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
)

type HTTP struct {
	server *httptest.Server
}

const msg = "Hello World\n"

func (h *HTTP) Setup() error {
	h.server = httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		rw.Write([]byte(msg))
	}))
	return nil
}

func (h *HTTP) Run() error {
	resp, err := http.Get(h.server.URL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	} else if resp.StatusCode != 200 || string(data) != msg {
		return fmt.Errorf("bad response: %d: %s", resp.StatusCode, data)
	}
	return nil
}

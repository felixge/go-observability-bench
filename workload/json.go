package workload

import (
	"encoding/json"
	"io/ioutil"
)

type JSONUnmarshal struct {
	File string `json:"file"`

	data []byte
}

func (j *JSONUnmarshal) Setup() error {
	data, err := ioutil.ReadFile(j.File)
	if err != nil {
		return err
	}
	j.data = data
	return nil
}

func (j *JSONUnmarshal) Run() error {
	var m interface{}
	err := json.Unmarshal(j.data, &m)
	return err
}

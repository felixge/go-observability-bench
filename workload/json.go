package workload

import (
	"encoding/json"
	"io/ioutil"
)

type JSON struct {
	File string `json:"file"`
	data []byte
}

func (j *JSON) Setup() error {
	data, err := ioutil.ReadFile(j.File)
	if err != nil {
		return err
	}
	j.data = data
	return nil
}

func (j *JSON) Run() error {
	var m interface{}
	if err := json.Unmarshal(j.data, &m); err != nil {
		return err
	}
	_, err := json.Marshal(m)
	return err
}

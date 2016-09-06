package tools

import (
	"testing"
	"encoding/json"
	"time"
	"fmt"
)

type example struct {
	Time Duration
}

func encode(item interface{}) []byte {
	res, err := json.Marshal(item)
	if err != nil {
		panic(err)
	}
	return res
}

func decode(data []byte, item interface{}) {
	err := json.Unmarshal(data, item)
	if err != nil {
		panic(err)
	}
}

func TestDuration(t *testing.T) {
	data := encode(&example{Duration{10 * time.Second}})
	fmt.Printf("%s\n", data)

	ex := &example{}
	decode(data, ex)

	ex2 := &example{}
	decode([]byte(`{"Time": "20s"}`), ex2)
	Equals(t, 20 * time.Second, ex2.Time.Duration)

	fmt.Printf("%+v\n", ex)

	ex.Time.Duration = 20 * time.Second
	fmt.Printf("%+v\n", ex)
}

package backends

import "encoding/json"

// encode and decode functions, the encode function will panic if the json marshalling fails.
func Encode(item interface{}) []byte {
	res, err := json.Marshal(item)
	if err != nil {
		panic(err)
	}
	return res
}

func Decode(data []byte, item interface{}) {
	err := json.Unmarshal(data, item)
	if err != nil {
		panic(err)
	}
}

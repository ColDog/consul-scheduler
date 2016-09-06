package tools

import (
	"time"
	"encoding/json"
	"fmt"
)
// taken from: https://github.com/golang/go/issues/4712#event-277683197
// since this is on the unplanned milestone currently, create my own method.
type Duration struct {
	time.Duration
}

func (d Duration) IsNone() bool {
	return d.Duration == 0
}

func (d *Duration) UnmarshalJSON(b []byte) (err error) {
	if b[0] == '"' {
		sd := string(b[1 : len(b)-1])
		d.Duration, err = time.ParseDuration(sd)
		return
	}

	var id int64
	id, err = json.Number(string(b)).Int64()
	d.Duration = time.Duration(id)

	return
}

func (d Duration) MarshalJSON() (b []byte, err error) {
	return []byte(fmt.Sprintf(`"%s"`, d.String())), nil
}

package ticktick

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// flexInt unmarshals JSON numbers that may be integer or float (e.g. 1 vs 1.0).
type flexInt int

func (f flexInt) Int() int { return int(f) }

func (f *flexInt) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		*f = 0
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(b, &n); err != nil {
		return err
	}
	if i, err := n.Int64(); err == nil {
		*f = flexInt(i)
		return nil
	}
	fl, err := n.Float64()
	if err != nil {
		return fmt.Errorf("flexInt: %w", err)
	}
	*f = flexInt(int64(fl))
	return nil
}

func (f flexInt) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Itoa(int(f))), nil
}

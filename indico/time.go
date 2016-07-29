package indico

import (
	"encoding/json"
	"time"
)

type indicoTime struct {
	time.Time
}

func (t *indicoTime) UnmarshalJSON(data []byte) error {
	var raw struct {
		Date     string `json:"date"`
		TimeZone string `json:"tz"`
		Time     string `json:"time"`
	}
	err := json.Unmarshal(data, &raw)
	if err != nil {
		return err
	}
	loc, err := time.LoadLocation(raw.TimeZone)
	if err != nil {
		return err
	}

	t.Time, err = time.ParseInLocation("2006-01-02 15:04:05", raw.Date+" "+raw.Time, loc)
	return err
}

type indicoDuration struct {
	time.Duration
}

func (d *indicoDuration) UnmarshalJSON(data []byte) error {
	var minutes int64
	err := json.Unmarshal(data, &minutes)
	if err != nil {
		return err
	}
	d.Duration = time.Duration(minutes) * time.Minute
	return nil
}

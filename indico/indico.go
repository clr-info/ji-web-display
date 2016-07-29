// Copyright ©2016 The ji-web-display Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package indico

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"
)

type TimeTable struct {
	ID   int    `json:"id"`
	URL  string `json:"url"`
	Days []Day  `json:"days"`
}

type Day struct {
	Date     time.Time `json:"date"`
	Sessions []Session `json:"sessions"`
}

type EntryID struct {
	ID          string        `json:"id"`
	Title       string        `json:"title"`
	Description string        `json:"description"`
	Location    string        `json:"location"`
	Room        string        `json:"room"`
	StartDate   time.Time     `json:"startDate"`
	EndDate     time.Time     `json:"endDate"`
	Duration    time.Duration `json:"duration"`
}

func (eid *EntryID) UnmarshalJSON(data []byte) error {
	var raw struct {
		ID          string         `json:"id"`
		Title       string         `json:"title"`
		Description string         `json:"description"`
		Location    string         `json:"location"`
		Room        string         `json:"room"`
		StartDate   indicoTime     `json:"startDate"`
		EndDate     indicoTime     `json:"endDate"`
		Duration    indicoDuration `json:"duration"`
	}
	err := json.Unmarshal(data, &raw)
	if err != nil {
		return err
	}

	*eid = EntryID{
		ID:          raw.ID,
		Title:       raw.Title,
		Description: raw.Description,
		Location:    raw.Location,
		Room:        raw.Room,
		StartDate:   raw.StartDate.Time,
		EndDate:     raw.EndDate.Time,
		Duration:    raw.Duration.Duration,
	}
	return nil
}

func (eid *EntryID) sanitize() {
	if eid.Duration.Seconds() == 0 {
		eid.Duration = eid.EndDate.Sub(eid.StartDate)
	}
}

type Session struct {
	EntryID
	Contributions []Contribution `json:"entries,omitempty"`
}

type Contribution struct {
	EntryID
	Material   []interface{} `json:"material"`
	URL        string        `json:"url"`
	Presenters []Presenter   `json:"presenters"`
}

type Presenter struct {
	Type        string `json:"_type"`
	Name        string `json:"name"`
	Affiliation string `json:"affiliation"`
	Email       string `json:"email"`
}

type timetableResponse struct {
	Count     int              `json:"count"`
	Type      string           `json:"_type"`
	Complete  bool             `json:"complete"`
	URL       string           `json:"url"`
	Timestamp int64            `json:"ts"`
	Results   timetableResults `json:"results"`
}

type timetableResults map[string]map[string]map[string]json.RawMessage

func FetchTimeTable(host string, evtid int) (*TimeTable, error) {
	url := fmt.Sprintf(
		"https://%s/export/timetable/%d.json?pretty=yes",
		host, evtid,
	)

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var raw timetableResponse
	err = json.NewDecoder(resp.Body).Decode(&raw)
	if err != nil {
		return nil, err
	}

	mdays, ok := raw.Results[strconv.Itoa(evtid)]
	if !ok || len(mdays) == 0 {
		return nil, fmt.Errorf("indico: no event with id=%d", evtid)
	}

	tbl := &TimeTable{
		ID:   evtid,
		URL:  raw.URL,
		Days: make([]Day, 0, len(mdays)),
	}

	for k, mday := range mdays {
		day := Day{
			Sessions: make([]Session, 0, len(mday)),
		}
		day.Date, err = time.Parse("20060102", k)
		if err != nil {
			return nil, err
		}
		for sid, msession := range mday {
			var v struct {
				EntryID
				Contributions map[string]Contribution `json:"entries,omitempty"`
			}
			switch sid[0] {
			case 's':
				err = json.Unmarshal(msession, &v)
			case 'b':
				err = json.Unmarshal(msession, &v.EntryID)
				if err != nil {
					return nil, err
				}
			default:
				log.Panicf("indico: unknown session id type %q\n", sid)
			}
			if err != nil {
				return nil, err
			}
			session := Session{
				EntryID: v.EntryID,
			}
			session.EntryID.sanitize()
			if len(v.Contributions) > 0 {
				session.Contributions = make([]Contribution, 0, len(v.Contributions))
				for _, vv := range v.Contributions {
					vv.EntryID.sanitize()
					session.Contributions = append(session.Contributions, vv)
				}
			}
			day.Sessions = append(day.Sessions, session)
		}
		tbl.Days = append(tbl.Days, day)
	}

	return tbl, nil
}

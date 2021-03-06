// Copyright ©2016 The ji-web-display Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package indico

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"
)

type TimeTable struct {
	ID   int
	URL  string
	Days []Day
}

type Day struct {
	Date     time.Time
	Sessions []Session
}

type EntryID struct {
	ID          string
	Title       string
	Description string
	Location    string
	Room        string
	StartDate   time.Time
	EndDate     time.Time
	Duration    time.Duration
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

func (s *Session) UnmarshalJSON(data []byte) error {
	var raw struct {
		ID            string                  `json:"id"`
		Title         string                  `json:"title"`
		Description   string                  `json:"description"`
		Location      string                  `json:"location"`
		Room          string                  `json:"room"`
		StartDate     indicoTime              `json:"startDate"`
		EndDate       indicoTime              `json:"endDate"`
		Duration      indicoDuration          `json:"duration"`
		Contributions map[string]Contribution `json:"entries"`
	}
	err := json.Unmarshal(data, &raw)
	if err != nil {
		return err
	}

	s.EntryID = EntryID{
		ID:          raw.ID,
		Title:       raw.Title,
		Description: raw.Description,
		Location:    raw.Location,
		Room:        raw.Room,
		StartDate:   raw.StartDate.Time,
		EndDate:     raw.EndDate.Time,
		Duration:    raw.Duration.Duration,
	}
	s.Contributions = nil
	for _, c := range raw.Contributions {
		c.sanitize()
		s.Contributions = append(s.Contributions, c)
	}
	return nil
}

type Contribution struct {
	EntryID
	URL        string
	Presenters []Presenter
}

func (c *Contribution) UnmarshalJSON(data []byte) error {
	var raw struct {
		ID          string         `json:"id"`
		Title       string         `json:"title"`
		Description string         `json:"description"`
		Location    string         `json:"location"`
		Room        string         `json:"room"`
		StartDate   indicoTime     `json:"startDate"`
		EndDate     indicoTime     `json:"endDate"`
		Duration    indicoDuration `json:"duration"`
		URL         string         `json:"url"`
		Presenters  []Presenter    `json:"presenters"`
	}
	err := json.Unmarshal(data, &raw)
	if err != nil {
		return err
	}

	c.EntryID = EntryID{
		ID:          raw.ID,
		Title:       raw.Title,
		Description: raw.Description,
		Location:    raw.Location,
		Room:        raw.Room,
		StartDate:   raw.StartDate.Time,
		EndDate:     raw.EndDate.Time,
		Duration:    raw.Duration.Duration,
	}
	c.URL = raw.URL
	c.Presenters = raw.Presenters
	return nil
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
		return nil, fmt.Errorf("could not GET timetable: %v", err)
	}
	defer resp.Body.Close()

	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("could not read all response body: %v", err)
	}

	tbl := TimeTable{ID: evtid}
	err = json.Unmarshal(buf, &tbl)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal JSON response: %v", err)
	}

	return &tbl, err
}

func (tbl *TimeTable) UnmarshalJSON(data []byte) error {
	var raw timetableResponse
	err := json.Unmarshal(data, &raw)
	if err != nil {
		return err
	}

	evtid := tbl.ID
	mdays, ok := raw.Results[strconv.Itoa(evtid)]
	if !ok || len(mdays) == 0 {
		return fmt.Errorf("indico: no event with id=%d", evtid)
	}

	tbl.URL = raw.URL
	tbl.Days = make([]Day, 0, len(mdays))

	for k, mday := range mdays {
		day := Day{
			Sessions: make([]Session, 0, len(mday)),
		}
		for _, msession := range mday {
			var session Session
			err = json.Unmarshal(msession, &session)
			if err != nil {
				return err
			}
			session.sanitize()
			day.Sessions = append(day.Sessions, session)
		}
		loc := time.UTC
		if len(day.Sessions) > 0 {
			loc = day.Sessions[0].StartDate.Location()
		}
		day.Date, err = time.ParseInLocation("20060102", k, loc)
		if err != nil {
			return err
		}
		tbl.Days = append(tbl.Days, day)
	}

	return nil
}

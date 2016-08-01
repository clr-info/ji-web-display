// Copyright ©2016 The ji-web-display Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"sort"
	"strings"
	"time"

	"github.com/clr-info/ji-web-display/indico"
)

type Agenda struct {
	Day      string
	Sessions []Session
}

type Session struct {
	Title         string
	Room          string
	Start, Stop   string
	Contributions []Contribution
	active        bool
}

func (s Session) Active() string {
	if s.active {
		return "current-session"
	}
	return ""
}

type Contribution struct {
	Title      string
	Start      string
	Stop       string
	Duration   time.Duration
	Presenters []Presenter
	active     bool
}

func (c Contribution) Active() string {
	if c.active {
		return "current-contribution"
	}
	return ""
}

type Presenter struct {
	Name        string
	Affiliation string
	Email       string
}

func (p Presenter) toHTML() string {
	o := p.Name
	if p.Affiliation != "" {
		o += " (<em>" + p.Affiliation + "</em>)"
	}
	return o
}

func displayPresenters(p []Presenter) string {
	var o []string
	for i, v := range p {
		if i > 0 {
			o = append(o, ", ")
		}
		o = append(o, v.toHTML())
	}
	return strings.Join(o, "")
}

func newAgenda(date time.Time, table *indico.TimeTable) Agenda {
	var day *indico.Day
	for i, d := range table.Days {
		if date.YearDay() == d.Date.YearDay() {
			day = &table.Days[i]
		}
	}

	agenda := Agenda{
		Day:      date.Format("2006-01-02 -- 15:04:05"),
		Sessions: make([]Session, 0, len(day.Sessions)),
	}

	sort.Sort(sessionsByDays(day.Sessions))
	for _, s := range day.Sessions {
		sort.Sort(contrByTime(s.Contributions))
		var contr []Contribution
		activeSession := date.Before(s.EndDate) && date.After(s.StartDate)
		for _, c := range s.Contributions {
			if !activeSession {
				continue
			}
			if c.EndDate.Before(date) {
				continue
			}
			var p []Presenter
			for _, pp := range c.Presenters {
				p = append(p, Presenter{
					Name:        pp.Name,
					Affiliation: pp.Affiliation,
					Email:       pp.Email,
				})
			}
			activeContr := date.Before(c.EndDate) && date.After(c.StartDate)
			contr = append(contr, Contribution{
				Title:      c.Title,
				Start:      c.StartDate.Format("15:04"),
				Stop:       c.EndDate.Format("15:04"),
				Duration:   c.Duration,
				Presenters: p,
				active:     activeContr,
			})
		}
		agenda.Sessions = append(agenda.Sessions, Session{
			Title:         s.Title,
			Room:          s.Room,
			Start:         s.StartDate.Format("15:04"),
			Stop:          s.EndDate.Format("15:04"),
			Contributions: contr,
			active:        activeSession,
		})
	}
	return agenda
}

type byDays []indico.Day

func (d byDays) Len() int           { return len(d) }
func (d byDays) Less(i, j int) bool { return d[i].Date.Unix() < d[j].Date.Unix() }
func (d byDays) Swap(i, j int)      { d[i], d[j] = d[j], d[i] }

type sessionsByDays []indico.Session

func (p sessionsByDays) Len() int           { return len(p) }
func (p sessionsByDays) Less(i, j int) bool { return p[i].StartDate.Unix() < p[j].StartDate.Unix() }
func (p sessionsByDays) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

type contrByTime []indico.Contribution

func (p contrByTime) Len() int           { return len(p) }
func (p contrByTime) Less(i, j int) bool { return p[i].StartDate.Unix() < p[j].StartDate.Unix() }
func (p contrByTime) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

var fakeData = Agenda{
	Day: "Today",
	Sessions: []Session{
		{
			Title: "Dev-1",
			Room:  "Room-1",
			Contributions: []Contribution{
				{
					Title:    "Title-1",
					Duration: 20 * time.Minute,
					Presenters: []Presenter{
						{
							Name:        "Dr Toto",
							Affiliation: "Navire Amiral",
							Email:       "toto@in2p3.fr",
						},
						{
							Name:        "Mr Tata",
							Affiliation: "Fregate",
							Email:       "tata@in2p3.fr",
						},
					},
				},
				{
					Title:    "Title-2",
					Duration: 20 * time.Minute,
				},
			},
		},
		{
			Title: "Dev-2",
			Room:  "Room-2",
			Contributions: []Contribution{
				{
					Title:    "Atelier-1",
					Duration: 90 * time.Minute,
				},
			},
		},
	},
}

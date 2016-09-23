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

func (s Session) CSSClass() string {
	if s.active {
		return "current-session"
	}
	return "session"
}

type Contribution struct {
	Title      string
	Start      string
	Stop       string
	Duration   time.Duration
	Presenters []Presenter
	active     bool
}

func (c Contribution) CSSClass() string {
	if c.active {
		return "current-contribution"
	}
	return "contribution"
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
		Day: date.Format("2006-01-02<br>15:04:05"),
	}

	if day == nil {
		return agenda
	}

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
	trimPastSessions(&agenda)
	trimActiveSessions(&agenda)
	trimFutureSessions(&agenda)
	return agenda
}

// trimPastSessions removes unnecessary past sessions
func trimPastSessions(agenda *Agenda) {
	idx := -1
	for i, s := range agenda.Sessions {
		if s.active {
			idx = i
			break
		}
	}
	const head = 1
	if idx > head {
		i := idx - head
		if i < 0 {
			i = head
		}
		agenda.Sessions = agenda.Sessions[i:]
	}
}

// trimActiveSessions removes unnecessary contributions of active sessions
func trimActiveSessions(agenda *Agenda) {
	for ii, s := range agenda.Sessions {
		if !s.active {
			continue
		}
		idx := 0
		for i, c := range s.Contributions {
			if c.active {
				idx = i
			}
		}
		if len(s.Contributions)-idx > 3 {
			i := idx + 3
			merged := Contribution{
				Title: " ... ",
				Start: s.Contributions[i].Start,
				Stop:  s.Contributions[len(s.Contributions)-1].Stop,
			}
			var sum time.Duration
			for j := i; j < len(s.Contributions); j++ {
				sum += s.Contributions[j].Duration
			}
			merged.Duration = sum
			s.Contributions = s.Contributions[:i]
			s.Contributions = append(s.Contributions, merged)
			agenda.Sessions[ii] = s
		}
	}
}

// trimFutureSessions removes unnecessary future sessions
func trimFutureSessions(agenda *Agenda) {
	idx := len(agenda.Sessions)
	for i, s := range agenda.Sessions {
		if s.active {
			idx = i
		}
	}
	if len(agenda.Sessions)-idx > 4 {
		i := idx + 4
		merged := Session{
			Title: " ... ",
			Start: agenda.Sessions[i].Start,
			Stop:  agenda.Sessions[len(agenda.Sessions)-1].Stop,
		}
		agenda.Sessions = agenda.Sessions[:i]
		agenda.Sessions = append(agenda.Sessions, merged)
	}
}

func sortTimeTable(tbl *indico.TimeTable) {
	if tbl == nil || tbl.Days == nil {
		return
	}

	sort.Sort(byDays(tbl.Days))
	for i, day := range tbl.Days {
		sort.Sort(sessionsByDays(day.Sessions))
		for j, sess := range day.Sessions {
			sort.Sort(contrByTime(sess.Contributions))
			day.Sessions[j] = sess
		}
		tbl.Days[i] = day
	}
}

type byDays []indico.Day

func (d byDays) Len() int           { return len(d) }
func (d byDays) Less(i, j int) bool { return d[i].Date.Unix() < d[j].Date.Unix() }
func (d byDays) Swap(i, j int)      { d[i], d[j] = d[j], d[i] }

type sessionsByDays []indico.Session

func (p sessionsByDays) Len() int { return len(p) }
func (p sessionsByDays) Less(i, j int) bool {
	pi := p[i]
	pj := p[j]
	si := pi.StartDate.Unix()
	sj := pj.StartDate.Unix()
	if si != sj {
		return si < sj
	}
	ei := pi.EndDate.Unix()
	ej := pj.EndDate.Unix()
	if ei != ej {
		return ei < ej
	}
	return pi.ID < pj.ID
}
func (p sessionsByDays) Swap(i, j int) { p[i], p[j] = p[j], p[i] }

type contrByTime []indico.Contribution

func (p contrByTime) Len() int           { return len(p) }
func (p contrByTime) Less(i, j int) bool { return p[i].StartDate.Unix() < p[j].StartDate.Unix() }
func (p contrByTime) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

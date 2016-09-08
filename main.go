// Copyright ©2016 The ji-web-display Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"text/template"
	"time"

	"github.com/clr-info/ji-web-display/indico"
	"golang.org/x/net/websocket"
)

var (
	// FIXME(sbinet): remove
	devTest = flag.Bool("dev-test", false, "enable test development mode")
)

func main() {

	log.SetFlags(0)
	log.SetPrefix("ji-web-display: ")

	var (
		addr      = flag.String("addr", ":80", "[hostname|ip]:port for web server")
		evtid     = flag.Int("evtid", 12779, "event id")
		nowLayout = "2006-01-02 15:04:05"
		snow      = flag.String("now", "", "agenda time. format="+nowLayout)
		sloc      = flag.String("loc", "Europe/Paris", "agenda time location")
	)

	flag.Parse()

	var now time.Time
	switch *snow {
	case "":
		now = time.Now()
	default:
		loc, err := time.LoadLocation(*sloc)
		if err != nil {
			log.Fatal(err)
		}
		now, err = time.ParseInLocation(nowLayout, *snow, loc)
		if err != nil {
			log.Fatal(err)
		}
	}

	host, port, err := net.SplitHostPort(*addr)
	if err != nil {
		log.Fatal(err)
	}

	if host == "" {
		host = getHostIP()
	}

	var tbl *indico.TimeTable

	_, err = net.LookupIP("indico.in2p3.fr")
	if err != nil {
		log.Printf("error looking up 'indico.in2p3.fr': %v\n", err)
		log.Printf("loading cached table...\n")
		tbl, err = loadCachedTable(*evtid)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		tbl, err = indico.FetchTimeTable("indico.in2p3.fr", *evtid)
		if err != nil {
			log.Fatal(err)
		}
	}
	sortTimeTable(tbl)

	srv := newServer(host+":"+port, tbl, now)
	mux := http.NewServeMux()
	mux.Handle("/", srv)
	mux.Handle("/data", websocket.Handler(srv.dataHandler))
	mux.HandleFunc("/refresh-time", srv.refreshTime)
	mux.HandleFunc("/refresh-timetable", srv.refreshTableHandler)
	err = http.ListenAndServe(srv.Addr, mux)
	if err != nil {
		log.Fatal(err)
	}
}

type server struct {
	Addr string
	tmpl *template.Template

	reg registry

	timec  chan time.Time
	now    time.Time
	datac  chan []byte
	mu     sync.RWMutex
	ttable *indico.TimeTable
}

func newServer(addr string, timeTable *indico.TimeTable, now time.Time) *server {
	srv := &server{
		Addr: addr,
		tmpl: template.Must(template.Must(template.New("ji-web").Funcs(template.FuncMap{
			"displayP": displayPresenters,
		}).Parse(mainPage)).Parse(agendaTmpl)),
		reg:    newRegistry(),
		timec:  make(chan time.Time),
		now:    now,
		datac:  make(chan []byte),
		ttable: timeTable,
	}
	go srv.crawler()
	go srv.run()
	return srv
}

func (srv *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	srv.tmpl.Execute(w, srv)
}

func (srv *server) run() {
	for {
		select {
		case c := <-srv.reg.register:
			srv.reg.clients[c] = true
			log.Printf("new client: %v\n", c)

		case c := <-srv.reg.unregister:
			if _, ok := srv.reg.clients[c]; ok {
				delete(srv.reg.clients, c)
				close(c.datac)
				log.Printf("client disconnected [%v]\n", c.ws.LocalAddr())
			}

		case data := <-srv.datac:
			/*
				dataBuf := new(bytes.Buffer)
				err := json.NewEncoder(dataBuf).Encode(data)
				if err != nil {
					log.Printf("error marshalling data: %v\n", err)
					continue
				}
			*/
			for c := range srv.reg.clients {
				select {
				case c.datac <- data:
				default:
					close(c.datac)
					delete(srv.reg.clients, c)
				}
			}
		}
	}
}

func (srv *server) crawler() {
	beat := 1 * time.Second
	ticker := time.NewTicker(beat)
	defer ticker.Stop()

	// loc := srv.ttable.Days[0].Date.Location()
	// now := time.Date(2016, 9, 27, 10, 4, 50, 0, loc)

	now := srv.now

	for {
		select {
		case srv.now = <-srv.timec:
			now = srv.now
		case <-ticker.C:
			buf := new(bytes.Buffer)
			if *devTest {
				h := now.Hour()
				switch {
				case h >= 0 && h < 8:
					beat = 1 * time.Hour
				case h >= 8 && h <= 18:
					beat = 3 * time.Minute
				case h > 18 && h <= 22:
					beat = 30 * time.Minute
				case h > 22:
					beat = 1 * time.Hour
				}
			}
			now = now.Add(beat)
			srv.mu.RLock()
			data := newAgenda(now, srv.ttable)
			srv.mu.RUnlock()
			err := srv.tmpl.ExecuteTemplate(buf, "agenda", data)
			if err != nil {
				log.Fatal(err)
			}
			srv.datac <- buf.Bytes()
			if *devTest {
				layout := "2006-01-02 15:04 -0700"
				end, _ := time.Parse(layout, "2016-09-29 12:12 +0200")
				start, _ := time.Parse(layout, "2016-09-26 14:45 +0200")
				if now.After(end) || now.Before(start) {
					now = start.Add(10 * time.Second)
				}
			}
		}
	}
}
func (srv *server) dataHandler(ws *websocket.Conn) {
	c := &client{
		srv:   srv,
		reg:   &srv.reg,
		datac: make(chan []byte, 256),
		ws:    ws,
	}
	c.reg.register <- c
	defer c.Release()

	c.run()
}

func (srv *server) refreshTime(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "invalid http request", http.StatusBadRequest)
		return
	}
	go func() {
		log.Printf("refreshing server internal time...\n")
		srv.timec <- time.Now()
		log.Printf("refreshing server internal time... [done]\n")

	}()
}

func (srv *server) refreshTableHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "invalid http request", http.StatusBadRequest)
		return
	}

	srv.mu.Lock()
	defer srv.mu.Unlock()

	id := srv.ttable.ID
	log.Printf("refreshing timetable-%d...\n", id)
	tbl, err := indico.FetchTimeTable("indico.in2p3.fr", srv.ttable.ID)
	if err != nil {
		log.Printf("error fetching timetable-%d: %v\n", id, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	srv.ttable = tbl
	sortTimeTable(srv.ttable)
	log.Printf("refreshing timetable-%d... [done]\n", id)
	fmt.Fprintf(w, "timetable-%d refreshed\n", id)
}

type client struct {
	srv   *server
	reg   *registry
	ws    *websocket.Conn
	datac chan []byte
}

func (c *client) Release() {
	c.reg.unregister <- c
	c.ws.Close()
	c.reg = nil
	c.srv = nil
}

func (c *client) run() {
	for data := range c.datac {
		err := websocket.Message.Send(c.ws, string(data))
		if err != nil {
			log.Printf(
				"error sending data to [%v]: %v\n",
				c.ws.LocalAddr(),
				err,
			)
			break
		}
	}
}

type registry struct {
	clients    map[*client]bool
	register   chan *client
	unregister chan *client
}

func newRegistry() registry {
	return registry{
		clients:    make(map[*client]bool),
		register:   make(chan *client),
		unregister: make(chan *client),
	}
}

const mainPage = `<!DOCTYPE html>
<html>
	<head>
		<meta name="viewport" content="width=device-width, minimum-scale=1.0, initial-scale=1.0, user-scalable=yes">
		<meta charset="utf-8">
		<title>JI-2016 Web Display</title>
		<style>
			:host {
				display: block;
				box-sizing: border-box;
				text-align: center;
				margin: 5px;
				max-width: 250px;
				min-width: 200px;
			}
			body {
				font-family: 'Roboto', 'Helvetica Neue', Helvetica, Arial, sans-serif;
				font-weight: 300;
				background: rgba(77, 62, 42, 0.14) -webkit-linear-gradient(left bottom,  rgba(13, 77, 104, 0.55), rgba(238, 238, 238, 0.8)) no-repeat scroll 0px 0;
				background: rgba(77, 62, 42, 0.14)    -moz-linear-gradient(center top,   rgba(13, 77, 104, 0.75) 31%, #434343 101%) no-repeat scroll 0px 0;
				background: rgba(77, 62, 42, 0.14)         linear-gradient(to center top,rgba(13, 77, 104, 0.75), #434343) no-repeat scroll 0px 0;
				background-attachment: fixed;
			}
			.session-container {
				padding:    6px;
				color:      #fff;
				margin-bottom: 2px;
				border-radius: 5px 5px 5px 5px;
			}
			.session {
				background:  #394c50;
				text-shadow: 5px 2px 5px #000;
			}
			.current-session {
				background:  #c7a30a;
				text-shadow: 4px 3px 5px #000;
			}
			.contribution {
				background: #427777;
				color:      #ffffcc;
			}
			.current-contribution {
				background: #fcb72b;
			}
			.contribution-container {
				padding:    6px;
				margin-top: 1px;
				margin-bottom: 1px;
				border-radius: 5px 5px 5px 5px;
			}
			h3.contribution-container{
				padding:0px;
			}
			.clock {
				float: right;
				vertical-align : bottom;
				background: #111;
				color:      #fff;
				width: 200px;
				height: 80px;
				font-size: 200%;
				text-align: center;
				border-radius: 10px 10px 10px 10px;
			}
		</style>
		<script type="text/javascript">
		var sock = null;

		function update(data) {
			var doc = document.getElementById("agenda");
			doc.innerHTML = data;
		};

		window.onload = function() {
			sock = new WebSocket("ws://{{.Addr}}/data");
			sock.onmessage = function(event) {
				update(event.data);
			};
		};
		</script>
	</head>

	<body>
		<div id="agenda"></div>
	</body>
</html>
`

const agendaTmpl = `{{define "agenda"}}
<div id="agenda-day" class="clock">{{.Day}}</div>
<br style="clear:both;">
{{block "session" .Sessions}}{{end}}
{{end}}

{{define "session"}}
{{- range . }}
<h2 class="{{.CSSClass}} session-container">{{.Title}} ({{.Start}} - {{.Stop}}) {{if .Room | ne "" }}Room: {{.Room}}{{end}}</h2>
{{- range .Contributions}}
	<div class="{{.CSSClass}} contribution-container">
		<h3 class="{{.CSSClass}} contribution-container">{{.Start}} - {{.Stop}}</h3>
		<b>{{.Title}}</b> (<i>{{.Duration}}</i>)
		{{block "presenters" .Presenters}}{{end}}
	</div>
{{- end}}
{{- end}}
{{end}}

{{define "presenters"}}<p>{{displayP .}}</p>{{end}}
`

func getHostIP() string {
	host, err := os.Hostname()
	if err != nil {
		log.Fatalf("could not retrieve hostname: %v\n", err)
	}

	addrs, err := net.LookupIP(host)
	if err != nil {
		log.Fatalf("could not lookup hostname IP: %v\n", err)
	}

	for _, addr := range addrs {
		ipv4 := addr.To4()
		if ipv4 == nil {
			continue
		}
		return ipv4.String()
	}

	log.Fatalf("could not infer host IP")
	return ""
}

// Copyright ©2016 The ji-web-display Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"sort"
	"text/template"
	"time"

	"github.com/clr-info/ji-web-display/indico"
	"golang.org/x/net/websocket"
)

func main() {

	log.SetFlags(0)
	log.SetPrefix("ji-web-display: ")

	var (
		addr  = flag.String("addr", ":80", "[hostname|ip]:port for web server")
		evtid = flag.Int("evtid", 12779, "event id")
	)

	flag.Parse()

	host, port, err := net.SplitHostPort(*addr)
	if err != nil {
		log.Fatal(err)
	}

	if host == "" {
		host = getHostIP()
	}

	tbl, err := indico.FetchTimeTable("indico.in2p3.fr", *evtid)
	if err != nil {
		log.Fatal(err)
	}
	sort.Sort(byDays(tbl.Days))

	srv := newServer(host+":"+port, tbl)
	mux := http.NewServeMux()
	mux.Handle("/", srv)
	mux.Handle("/data", websocket.Handler(srv.dataHandler))
	err = http.ListenAndServe(srv.Addr, mux)
	if err != nil {
		log.Fatal(err)
	}
}

type server struct {
	Addr string
	tmpl *template.Template

	reg registry // clients interested in URLs

	datac  chan []byte
	ttable *indico.TimeTable
}

func newServer(addr string, timeTable *indico.TimeTable) *server {
	srv := &server{
		Addr: addr,
		tmpl: template.Must(template.Must(template.New("ji-web").Funcs(template.FuncMap{
			"displayP": displayPresenters,
		}).Parse(mainPage)).Parse(agendaTmpl)),
		reg:    newRegistry(),
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
	beat := 5 * time.Second
	ticker := time.NewTicker(beat)
	defer ticker.Stop()

	loc := srv.ttable.Days[0].Date.Location()
	now := time.Date(2016, 9, 27, 10, 04, 30, 0, loc)

	for {
		select {
		case <-ticker.C:
			buf := new(bytes.Buffer)
			now = now.Add(beat)
			data := newAgenda(now, srv.ttable)
			// data.Day += " (" + now.Format("2006-01-02 - 15:04:05") + ")"
			err := srv.tmpl.ExecuteTemplate(buf, "agenda", data)
			if err != nil {
				log.Fatal(err)
			}
			srv.datac <- buf.Bytes()
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
		<title>JI-2016 Web Display</title>
		<style>
			body {
				font-family: sans-serif;
			}
			h2 {
				color:      #fff;
				background: #034f84;
			}
			[id=current-session] {
				color:      #fff;
				background: #f7786b;
			}
			h3 {
				background: #92a8d1;
			}
			[id=current-contribution] {
				background: #f7cac9;
			}
			#sidebar {
				float: right;
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
<h1 id="agenda-day">{{.Day}}</h1>
{{block "session" .Sessions}}{{end}}
{{end}}

{{define "session"}}
{{- range . }}
<h2 id="{{.Active}}">{{.Title}} ({{.Start}} - {{.Stop}}) {{if .Room | ne "" }}Room: {{.Room}}{{end}}</h2>
{{- range .Contributions}}
	<div id="{{.Active}}">
		<h3 id="{{.Active}}">{{.Start}} - {{.Stop}}</h3>
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

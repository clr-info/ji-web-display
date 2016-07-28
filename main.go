package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"text/template"
	"time"

	"golang.org/x/net/websocket"
)

type Site struct {
	URL  string `json:"url"`
	Time int64  `json:"time"`
}

var (
	sites []Site
)

func main() {

	log.SetFlags(0)
	log.SetPrefix("ji-web-display: ")

	var addr = flag.String("addr", ":80", "[hostname|ip]:port for web server")

	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		log.Fatalf("ji-web-display myconfig.json\n")
	}
	rand.Seed(42)

	host, port, err := net.SplitHostPort(*addr)
	if err != nil {
		log.Fatal(err)
	}

	if host == "" {
		host = getHostIP()
	}

	in, err := ioutil.ReadFile(flag.Arg(0))
	if err != nil {
		log.Fatal(err)
	}

	err = json.Unmarshal(in, &sites)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(sites)

	done := make(chan bool)
	srv := newServer(host + ":" + port)

	go generate(srv.datac, done)

	mux := http.NewServeMux()
	mux.Handle("/", srv)
	mux.Handle("/data", websocket.Handler(srv.dataHandler))
	err = http.ListenAndServe(srv.Addr, mux)
	if err != nil {
		done <- true
		log.Fatal(err)
	}
}

type server struct {
	Addr    string
	Default string
	tmpl    *template.Template

	urlReg registry // clients interested in URLs

	datac chan []byte
}

func newServer(addr string) *server {
	srv := &server{
		Addr:    addr,
		Default: "http://in2p3.fr",
		tmpl:    template.Must(template.New("ji-web").Parse(page)),
		urlReg:  newRegistry(),
		datac:   make(chan []byte),
	}
	go srv.run()
	return srv
}

func (srv *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	srv.tmpl.Execute(w, srv)
}

func (srv *server) run() {
	for {
		select {
		case c := <-srv.urlReg.register:
			srv.urlReg.clients[c] = true
			log.Printf("new client: %v\n", c)

		case c := <-srv.urlReg.unregister:
			if _, ok := srv.urlReg.clients[c]; ok {
				delete(srv.urlReg.clients, c)
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
			for c := range srv.urlReg.clients {
				select {
				case c.datac <- data:
				default:
					close(c.datac)
					delete(srv.urlReg.clients, c)
				}
			}
		}
	}
}

func (srv *server) dataHandler(ws *websocket.Conn) {
	c := &client{
		srv:   srv,
		reg:   &srv.urlReg,
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
	//c.ws.SetReadLimit(maxMessageSize)
	//c.ws.SetReadDeadline(time.Now().Add(pongWait))
	//c.ws.SetPongHandler(func(string) error { c.ws.SetReadDeadline(time.Now().Add(pongWait)); return nil })
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

func generate(datac chan []byte, done chan bool) {
	for {
		site := sites[rand.Intn(len(sites))]
		data, err := json.Marshal(site)
		if err != nil {
			log.Fatal(err)
			continue
		}
		select {
		case datac <- data:
			duration := time.Duration(site.Time) * time.Second
			time.Sleep(duration)
		case <-done:
			return
		}
	}
}

const page = `
<html>
	<head>
		<title>JI-2016 Web Display</title>
		<script type="text/javascript">
		var sock = null;

		function update(url) {
			var doc = document.getElementById("site-frame");
			doc.src = url;
		};

		window.onload = function() {
			sock = new WebSocket("ws://{{.Addr}}/data");
			sock.onmessage = function(event) {
				var data = JSON.parse(event.data);
				console.log("--> ["+data.url+"]");
				update(data.url);
			};
		};
		</script>
	</head>

	<body style="overflow:hidden;">
		<div id="site-title"></div>
		<iframe id="site-frame" height=100% width=100% style="border:none;" src="{{.Default}}"></iframe>
	</body>
</html>
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

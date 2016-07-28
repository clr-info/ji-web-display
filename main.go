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
	"strconv"
	"time"

	"golang.org/x/net/websocket"
)

type Site struct {
	URL  string `json:"url"`
	Time int64  `json:"time"`
}

var (
	sites []Site
	datac = make(chan Site)
	host  = "127.0.0.1"
	port  = flag.Int("port", 80, "Web server port")
)

func main() {

	log.SetFlags(0)
	log.SetPrefix("ji-web-display: ")

	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		log.Fatalf("ji-web-display myconfig.json\n")
	}
	rand.Seed(42)

	host = getHostIP()

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
	go generate(datac, done)

	http.HandleFunc("/", pageHandle)
	http.Handle("/data", websocket.Handler(dataHandler))
	err = http.ListenAndServe(":"+strconv.Itoa(*port), nil)
	if err != nil {
		done <- true
		log.Fatal(err)
	}
}

func generate(datac chan Site, done chan bool) {
	for {
		site := sites[rand.Intn(len(sites))]
		select {
		case datac <- site:
			duration := time.Duration(site.Time) * time.Second
			time.Sleep(duration)
		case <-done:
			return
		}
	}
}

func pageHandle(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, page, host, *port)
}

func dataHandler(ws *websocket.Conn) {
	for data := range datac {
		err := websocket.JSON.Send(ws, data)
		if err != nil {
			log.Printf("error sending data: %v\n", err)
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
			sock = new WebSocket("ws://%s:%d/data");
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
		<iframe id="site-frame" height=100%% width=100%% style="border:none;" src=""></iframe>
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

# ji-web-display

A simple Go-based web server to display the [JI-2016](https://indico.in2p3.fr/event/12779) agenda.

## Examples

```shell
$> cat test.json
[
		{"url": "http://clrwww.in2p3.fr", "time": 10},
		{"url": "https://indico.in2p3.fr/event/12779/", "time": 10},
		{"url": "https://www.cern.ch", "time": 5},
		{"url": "https://golang.org", "time": 10},
]

$> ji-web-display -port=9090 ./test.json &
$> open http://127.0.0.1:9090
```




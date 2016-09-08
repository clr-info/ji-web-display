# ji-web-display

A simple Go-based web server to display the [JI-2016](https://indico.in2p3.fr/event/12779) agenda.

## Examples

```shell
$> go get github.com/clr-info/ji-web-display
$> ji-web-display -addr=:9090 -dev-test -now="2016-09-27 10:45:00" &
$> open http://127.0.0.1:9090
```


## Handlers

### /refresh-timetable

Manually refresh (and fetch from indico) the time table:

```sh
$> curl -X POST http://localhost:9090/refresh-timetable
timetable-12779 refreshed
```

### /refresh-timetable

Manually refresh the internal server time:

```sh
$> curl -X POST http://localhost:9090/refresh-time
time is now: 2016-09-08 14:05:53.177434474 +0100 BST
```

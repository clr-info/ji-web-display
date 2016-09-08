// Copyright ©2016 The ji-web-display Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build ignore

package main

import (
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

func main() {

	id := flag.Int("id", 12779, "timetable ID")
	host := flag.String("host", "indico.in2p3.fr", "Indico server")

	flag.Parse()

	url := fmt.Sprintf(
		"https://%s/export/timetable/%d.json?pretty=yes",
		*host, *id,
	)

	resp, err := http.Get(url)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	w := base64.NewEncoder(base64.StdEncoding, os.Stdout)
	defer w.Close()

	_, err = io.Copy(w, resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	err = w.Close()
	if err != nil {
		log.Fatal(err)
	}
}

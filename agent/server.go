package agent

import (
	log "github.com/Sirupsen/logrus"
	"io"
	"net/http"
)

func hello(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "OK\n")
}

func Serve() {
	http.HandleFunc("/", hello)
	log.Fatal(http.ListenAndServe(":8231", nil))
}

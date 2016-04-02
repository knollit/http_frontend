package main

import (
	"bytes"
	"net/http"
)

type httpMethods map[string]struct{}

func (methods httpMethods) permit(method string, w http.ResponseWriter) bool {
	if _, ok := methods[method]; !ok {
		buf := bytes.Buffer{}
		for m := range methods {
			buf.WriteString(m)
		}
		w.Header().Set("Allow", buf.String())
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return false
	}
	return true
}

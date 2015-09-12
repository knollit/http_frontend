package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"

	"github.com/golang/protobuf/proto"
	orgPB "github.com/mikeraimondi/api_service/organizations/proto"
)

func main() {
	server := &server{}
	defer func() {
		if err := server.Close(); err != nil {
			log.Println("Failed to close server: ", err)
		}
	}()

	log.Fatal(server.run(":80"))
}

type server struct{}

func (s *server) handler() http.Handler {
	return s.rootHandler()
}

func (s *server) run(addr string) error {
	httpServer := &http.Server{
		Addr:    addr,
		Handler: s.handler(),
	}

	log.Printf("Listening for requests on %s...\n", addr)
	return httpServer.ListenAndServe()
}

func (s *server) Close() error {
	return nil
}

func (s *server) rootHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			req := &orgPB.Request{
				Action: orgPB.Request_INDEX,
			}
			data, err := proto.Marshal(req)
			if err != nil {
				log.Printf("Request marshal error %v", err)
				http.Error(w, "Internal application error", http.StatusInternalServerError)
				return
			}
			conn, err := net.Dial("tcp", fmt.Sprintf("%v:13800", os.Getenv("ORGSVC_PORT_13800_TCP_ADDR")))
			if err != nil {
				log.Printf("Request error %v", err)
				http.Error(w, "Internal application error", http.StatusInternalServerError)
				return
			}
			defer conn.Close()
			conn.Write(data)
			buf := make([]byte, 1024)
			i, err := conn.Read(buf)
			if err != nil && err != io.EOF {
				log.Printf("Response error %v", err)
				http.Error(w, "Internal application error", http.StatusInternalServerError)
				return
			}
			orgsMsg := &orgPB.Organizations{}
			err = proto.Unmarshal(buf[:i], orgsMsg)
			if err != nil {
				log.Print(err)
				http.Error(w, "Internal application error", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			json.NewEncoder(w).Encode(orgsMsg)
			return
		} else if r.Method == "POST" {
			if err := r.ParseForm(); err != nil {
				http.Error(w, "Bad request", http.StatusBadRequest)
				return
			}
			req := &orgPB.Request{
				Action:       orgPB.Request_NEW,
				Organization: &orgPB.Organization{Name: r.Form.Get("name")},
			}
			data, err := proto.Marshal(req)
			if err != nil {
				log.Print(err)
				http.Error(w, "Internal application error", http.StatusInternalServerError)
				return
			}
			conn, err := net.Dial("tcp", fmt.Sprintf("%v:13800", os.Getenv("ORGSVC_PORT_13800_TCP_ADDR")))
			if err != nil {
				log.Printf("Request error %v", err)
				http.Error(w, "Internal application error", http.StatusInternalServerError)
				return
			}
			defer conn.Close()
			conn.Write(data)
			buf := make([]byte, 1024)
			i, err := conn.Read(buf)
			if err != nil && err != io.EOF {
				log.Printf("Response error %v", err)
				http.Error(w, "Internal application error", http.StatusInternalServerError)
				return
			}
			orgMsg := &orgPB.Organization{}
			err = proto.Unmarshal(buf[:i], orgMsg)
			if err != nil {
				log.Print(err)
				http.Error(w, "Internal application error", http.StatusInternalServerError)
				return
			}
			if len(orgMsg.Error) > 0 {
				http.Error(w, orgMsg.Error, http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusOK)
			return
		} else {
			w.Header().Set("Allow", "GET, POST")
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
}

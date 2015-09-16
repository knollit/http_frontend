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
	"github.com/mikeraimondi/api_service"
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
		req := &orgPB.Request{}
		if r.Method == "GET" {
			req.Action = orgPB.Request_INDEX
		} else if r.Method == "POST" {
			if err := r.ParseForm(); err != nil {
				http.Error(w, "Bad request", http.StatusBadRequest)
				return
			}
			req.Action = orgPB.Request_NEW
			req.Organization = &orgPB.Organization{Name: r.Form.Get("name")}
		} else {
			w.Header().Set("Allow", "GET, POST")
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
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
		apiService.WriteWithSize(conn, data)
		var response []*orgPB.Organization
		for {
			buf, err := apiService.ReadWithSize(conn)
			if err == io.EOF {
				break
			} else if err != nil {
				log.Printf("Response error %v", err)
				http.Error(w, "Internal application error", http.StatusInternalServerError)
				return
			}
			orgMsg := &orgPB.Organization{}
			err = proto.Unmarshal(buf, orgMsg)
			if err != nil {
				log.Print(err)
				http.Error(w, "Internal application error", http.StatusInternalServerError)
				return
			}
			if len(orgMsg.Error) > 0 {
				w.WriteHeader(http.StatusBadRequest)
			}
			response = append(response, orgMsg)
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if len(response) == 1 {
			json.NewEncoder(w).Encode(response[0])
			return
		}
		json.NewEncoder(w).Encode(response)
		return
	})
}

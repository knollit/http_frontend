package main

import (
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"

	"github.com/google/flatbuffers/go"
	"github.com/mikeraimondi/knollit/common"
	"github.com/mikeraimondi/knollit/http_frontend/organizations"
)

var (
	certPath = flag.String("cert-path", os.Getenv("TLS_CERT_PATH"), "Path to cert file")
	keyPath  = flag.String("key-path", os.Getenv("TLS_KEY_PATH"), "Path to private key file")
)

func main() {
	// Load client cert
	cert, err := tls.LoadX509KeyPair(*certPath, *keyPath)
	if err != nil {
		log.Fatal("Failed to open client cert and/or key: ", err)
	}

	tlsConf := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: true, //TODO dev only
		ClientSessionCache: tls.NewLRUClientSessionCache(1000),
	}
	server := &server{
		getOrgSvcConn: func() (net.Conn, error) {
			return tls.Dial("tcp", fmt.Sprintf("%v:13800", os.Getenv("ORGSVC_PORT_13800_TCP_ADDR")), tlsConf)
		},
	}

	defer func() {
		if err := server.Close(); err != nil {
			log.Println("Failed to close server: ", err)
		}
	}()

	log.Fatal(server.run(":80"))
}

type server struct {
	getOrgSvcConn func() (net.Conn, error)
}

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
		b := flatbuffers.NewBuilder(0)
		if r.Method == "GET" {
			organizations.OrganizationStart(b)
			organizations.OrganizationAddAction(b, organizations.ActionIndex)
		} else if r.Method == "POST" {
			if err := r.ParseForm(); err != nil {
				http.Error(w, "Bad request", http.StatusBadRequest)
				return
			}
			namePosition := b.CreateByteString([]byte(r.Form.Get("name")))
			organizations.OrganizationStart(b)
			organizations.OrganizationAddName(b, namePosition)
		} else {
			w.Header().Set("Allow", "GET, POST")
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		orgPosition := organizations.OrganizationEnd(b)
		b.Finish(orgPosition)

		conn, err := s.getOrgSvcConn()
		if err != nil {
			log.Printf("Request error %v", err)
			http.Error(w, "Internal application error", http.StatusInternalServerError)
			return
		}
		defer conn.Close()
		if _, err := common.WriteWithSize(conn, b.Bytes[b.Head():]); err != nil {
			log.Printf("Request error %v", err)
			http.Error(w, "Internal application error", http.StatusInternalServerError)
			return
		}
		var response []organization
		for {
			buf, _, err := common.ReadWithSize(conn)
			if err == io.EOF {
				break
			} else if err != nil {
				log.Printf("Response error %v", err)
				http.Error(w, "Internal application error", http.StatusInternalServerError)
				return
			}
			orgMsg := organizations.GetRootAsOrganization(buf, 0)
			if len(orgMsg.Error()) > 0 {
				w.WriteHeader(http.StatusBadRequest)
			}
			response = append(response, organizationFromFlatBuffer(orgMsg))
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

type organization struct {
	Name  string `json:"name"`
	Error string
}

func organizationFromFlatBuffer(org *organizations.Organization) organization {
	return organization{
		Name:  string(org.Name()),
		Error: string(org.Error()),
	}
}

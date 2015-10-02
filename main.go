package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/golang/protobuf/proto"
	"github.com/mikeraimondi/api_service"
	orgPB "github.com/mikeraimondi/api_service/organizations/proto"
)

var (
	caPath   = flag.String("ca-path", os.Getenv("TLS_CA_PATH"), "Path to CA file")
	certPath = flag.String("cert-path", os.Getenv("TLS_CERT_PATH"), "Path to cert file")
	keyPath  = flag.String("key-path", os.Getenv("TLS_KEY_PATH"), "Path to private key file")
)

func main() {
	// Load client cert
	cert, err := tls.LoadX509KeyPair(*certPath, *keyPath)
	if err != nil {
		log.Fatal("Failed to open client cert and/or key: ", err)
	}

	// Load CA cert
	caCert, err := ioutil.ReadFile(*caPath)
	if err != nil {
		log.Fatal("Failed to open CA cert: ", err)
	}
	caCertPool := x509.NewCertPool()
	if ok := caCertPool.AppendCertsFromPEM(caCert); !ok {
		log.Fatal("Failed to parse CA cert")
	}

	server := &server{
		TLSConf: &tls.Config{
			Certificates:       []tls.Certificate{cert},
			RootCAs:            caCertPool,
			InsecureSkipVerify: true, //TODO dev only
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
	TLSConf *tls.Config
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
		conn, err := tls.Dial("tcp", fmt.Sprintf("%v:13800", os.Getenv("ORGSVC_PORT_13800_TCP_ADDR")), s.TLSConf)
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

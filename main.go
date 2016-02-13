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
	"os/signal"
	"sync"
	"syscall"

	"github.com/google/flatbuffers/go"
	"github.com/gorilla/mux"
	"github.com/knollit/http_frontend/endpoints"
	"github.com/knollit/http_frontend/organizations"
	"github.com/mikeraimondi/prefixedio"
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
	s := newServer()
	s.getOrgSvcConn = func() (net.Conn, error) {
		// TODO what if multiple goroutines call this?
		return tls.Dial("tcp", fmt.Sprintf("%v:13800", os.Getenv("ORGSVC_PORT_13800_TCP_ADDR")), tlsConf)
	}
	s.getEndpointSvcConn = func() (net.Conn, error) {
		// TODO what if multiple goroutines call this?
		return tls.Dial("tcp", fmt.Sprintf("%v:13800", os.Getenv("ENDPOINTSVC_PORT_13800_TCP_ADDR")), tlsConf)
	}

	defer func() {
		if err := s.Close(); err != nil {
			log.Println("Failed to close server: ", err)
		}
	}()

	errChan := make(chan error)
	exitChan := make(chan os.Signal)
	signal.Notify(exitChan, syscall.SIGINT, syscall.SIGTERM)

	go s.run(":80", errChan)

	select {
	case err = <-errChan:
		log.Println("Error starting listener: ", err)
		return
	case exit := <-exitChan:
		log.Println("Exiting: ", exit)
		return
	}
}

func newServer() *server {
	return &server{
		builderPool: sync.Pool{
			New: func() interface{} {
				return flatbuffers.NewBuilder(0)
			},
		},
		prefixedBufPool: sync.Pool{
			New: func() interface{} {
				return &prefixedio.Buffer{}
			},
		},
	}
}

type server struct {
	getOrgSvcConn      func() (net.Conn, error)
	getEndpointSvcConn func() (net.Conn, error)
	builderPool        sync.Pool
	prefixedBufPool    sync.Pool
}

func (s *server) handler() http.Handler {
	r := mux.NewRouter()
	r.HandleFunc("/organizations", s.organizationsHandler)
	r.HandleFunc("/organizations/{organizationName}/endpoints/{endpointID}", s.endpointHandler)
	r.HandleFunc("/health_check", s.healthCheckHandler)
	return r
}

func (s *server) run(addr string, errChan chan error) {
	httpServer := &http.Server{
		Addr:    addr,
		Handler: s.handler(),
	}

	log.Printf("Listening for requests on %s...\n", addr)
	errChan <- httpServer.ListenAndServe()
}

func (s *server) Close() error {
	return nil
}

func (s *server) endpointHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	b := s.builderPool.Get().(*flatbuffers.Builder)
	defer s.builderPool.Put(b)

	orgConn, err := s.getOrgSvcConn()
	if err != nil {
		log.Printf("Request error %v", err)
		http.Error(w, "Internal application error", http.StatusInternalServerError)
		return
	}
	defer orgConn.Close()
	organization := &organization{
		Name:   vars["organizationName"],
		action: organizations.ActionRead,
	}
	if _, err := prefixedio.WriteBytes(orgConn, organization.toFlatBufferBytes(b)); err != nil {
		log.Printf("Request error %v", err)
		http.Error(w, "Internal application error", http.StatusInternalServerError)
		return
	}

	buf := s.prefixedBufPool.Get().(*prefixedio.Buffer)
	defer s.prefixedBufPool.Put(buf)
	_, err = buf.ReadFrom(orgConn)
	if err != nil {
		log.Printf("Request error %v", err)
		http.Error(w, "Internal application error", http.StatusInternalServerError)
		return
	}
	// TODO read org
	// orgMsg := organizations.GetRootAsOrganization(buf, 0)

	endpointConn, err := s.getEndpointSvcConn()
	if err != nil {
		log.Printf("Request error %v", err)
		http.Error(w, "Internal application error", http.StatusInternalServerError)
		return
	}
	defer endpointConn.Close()
	endpoint := &endpoint{
		ID: vars["endpointID"],
		// TODO action
	}
	if _, err := prefixedio.WriteBytes(endpointConn, endpoint.toFlatBufferBytes(b)); err != nil {
		log.Printf("Request error %v", err)
		http.Error(w, "Internal application error", http.StatusInternalServerError)
		return
	}
	_, err = buf.ReadFrom(endpointConn)
	if err != nil {
		log.Printf("Request error %v", err)
		http.Error(w, "Internal application error", http.StatusInternalServerError)
		return
	}
	endpointMsg := endpoints.GetRootAsEndpoint(buf.Bytes(), 0)
	endpoint.URL = string(endpointMsg.URL())
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(endpoint)
	return
}

func (s *server) organizationsHandler(w http.ResponseWriter, r *http.Request) {
	b := s.builderPool.Get().(*flatbuffers.Builder)
	defer s.builderPool.Put(b)

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
	if _, err := prefixedio.WriteBytes(conn, b.Bytes[b.Head():]); err != nil {
		log.Printf("Request error %v", err)
		http.Error(w, "Internal application error", http.StatusInternalServerError)
		return
	}
	response := []organization{}
	buf := s.prefixedBufPool.Get().(*prefixedio.Buffer)
	defer s.prefixedBufPool.Put(buf)
	for {
		_, err = buf.ReadFrom(conn)
		if err == io.EOF {
			break
		} else if err != nil {
			log.Printf("Response error %v", err)
			http.Error(w, "Internal application error", http.StatusInternalServerError)
			return
		}
		orgMsg := organizations.GetRootAsOrganization(buf.Bytes(), 0)
		if len(orgMsg.Error()) > 0 {
			w.WriteHeader(http.StatusBadRequest)
		} else if r.Method == "POST" {
			w.WriteHeader(http.StatusCreated)
		}
		response = append(response, organizationFromFlatBuffer(orgMsg))
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(response)
}

func (s *server) healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	// TODO include DB check
	conn, err := s.getOrgSvcConn()
	if err != nil {
		http.Error(w, "Organizations unavailable", http.StatusServiceUnavailable)
		return
	}
	conn.Close()
	conn, err = s.getEndpointSvcConn()
	if err != nil {
		http.Error(w, "Endpoints unavailable", http.StatusServiceUnavailable)
		return
	}
	conn.Close()
	w.WriteHeader(http.StatusNoContent)
}

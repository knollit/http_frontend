package main

import (
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/gorilla/mux"
	"github.com/knollit/http_frontend/endpoints"
	"github.com/knollit/http_frontend/organizations"
)

var (
	certPath = flag.String("cert-path", os.Getenv("TLS_CERT_PATH"), "Path to cert file")
	keyPath  = flag.String("key-path", os.Getenv("TLS_KEY_PATH"), "Path to private key file")
)

const (
	contentTypeHeader    = "Content-Type"
	jsonContentTypeValue = "application/json; charset=utf-8"
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
	s := &server{}
	s.servicePool = sync.Pool{
		New: func() interface{} {
			return newService(s)
		},
	}
	return s
}

type server struct {
	getOrgSvcConn      func() (net.Conn, error)
	getEndpointSvcConn func() (net.Conn, error)
	servicePool        sync.Pool
}

func (s *server) getService() *service {
	return s.servicePool.Get().(*service)
}

func (s *server) putService(svc *service) {
	svc.reset()
	s.servicePool.Put(svc)
}

func (s *server) handler() http.Handler {
	r := mux.NewRouter()
	r.HandleFunc("/organizations", s.organizationsHandler)
	r.HandleFunc("/organizations/{organizationName}/endpoints", s.endpointsHandler)
	r.HandleFunc("/organizations/{organizationName}/endpoints/{endpointID}", s.endpointsHandler)
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

func (s *server) endpointsHandler(w http.ResponseWriter, r *http.Request) {
	ok := httpMethods{
		http.MethodGet:  {},
		http.MethodPost: {},
	}.permit(r.Method, w)
	if !ok {
		return
	}

	vars := mux.Vars(r)
	svc := s.getService()
	defer s.putService(svc)
	thisEndpoint := &endpoint{}
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
		thisEndpoint.URL = r.Form.Get("url")
		thisEndpoint.Action = endpoints.ActionNew
	} else if r.Method == http.MethodGet {
		thisEndpoint.ID = vars["endpointID"]
		thisEndpoint.Action = endpoints.ActionRead
	}

	org := &organization{
		Name:   vars["organizationName"],
		action: organizations.ActionRead,
	}
	orgs, err := svc.sync(org)
	if err != nil {
		log.Printf("org request error %v", err)
		http.Error(w, "internal application error", http.StatusInternalServerError)
		return
	}
	orgResp := orgs[0].(*organization)
	if thisEndpoint.OrganizationID = orgResp.Name; len(thisEndpoint.OrganizationID) == 0 {
		// TODO 404?
		log.Println("no organization ID returned")
		http.Error(w, "internal application error", http.StatusInternalServerError)
		return
	}

	endpointResponses, err := svc.sync(thisEndpoint)
	if err != nil {
		log.Printf("org request error %v", err)
		http.Error(w, "internal application error", http.StatusInternalServerError)
		return
	}
	endpointResponse := endpointResponses[0].(*endpoint)
	if endpointResponse.err != nil {
		w.WriteHeader(http.StatusNotFound)
	} else if r.Method == http.MethodPost {
		w.WriteHeader(http.StatusCreated)
	}

	w.Header().Set(contentTypeHeader, jsonContentTypeValue)
	json.NewEncoder(w).Encode(endpointResponse)
}

func (s *server) organizationsHandler(w http.ResponseWriter, r *http.Request) {
	ok := httpMethods{
		http.MethodGet:  {},
		http.MethodPost: {},
	}.permit(r.Method, w)
	if !ok {
		return
	}

	svc := s.getService()
	defer s.putService(svc)
	org := &organization{}

	if r.Method == http.MethodGet {
		org.action = organizations.ActionIndex
	} else if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
		org.Name = r.Form.Get("name")
	}

	orgs, err := svc.sync(org)
	if err != nil {
		log.Printf("org request error %v", err)
		http.Error(w, "internal application error", http.StatusInternalServerError)
		return
	}
	valid := true
	for _, orgResp := range orgs {
		orgRes := orgResp.(*organization)
		if orgRes.err != nil {
			valid = false
			break
		}
	}
	if !valid {
		w.WriteHeader(http.StatusBadRequest)
	} else if r.Method == http.MethodPost {
		w.WriteHeader(http.StatusCreated)
	}
	w.Header().Set(contentTypeHeader, jsonContentTypeValue)
	json.NewEncoder(w).Encode(orgs)
}

func (s *server) healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	// TODO include DB check
	conn, err := s.getOrgSvcConn()
	if err != nil {
		log.Println("health check error in organization service: ", err)
		http.Error(w, "organizations unavailable", http.StatusServiceUnavailable)
		return
	}
	conn.Close()
	conn, err = s.getEndpointSvcConn()
	if err != nil {
		log.Println("health check error in endpoint service: ", err)
		http.Error(w, "endpoints unavailable", http.StatusServiceUnavailable)
		return
	}
	conn.Close()
	log.Println("health check OK")
	w.WriteHeader(http.StatusNoContent)
}

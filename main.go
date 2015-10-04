package main

import (
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/boltdb/bolt"
	"github.com/golang/protobuf/proto"
	"github.com/mikeraimondi/api_service"
	orgPB "github.com/mikeraimondi/api_service/organizations/proto"
)

var (
	certPath = flag.String("cert-path", os.Getenv("TLS_CERT_PATH"), "Path to cert file")
	keyPath  = flag.String("key-path", os.Getenv("TLS_KEY_PATH"), "Path to private key file")
)

const tlsSessionBucket = "TLSSessionCache"

func main() {
	// Load client cert
	cert, err := tls.LoadX509KeyPair(*certPath, *keyPath)
	if err != nil {
		log.Fatal("Failed to open client cert and/or key: ", err)
	}

	db, err := bolt.Open("json_api.db", 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		log.Fatal("Failed to open DB: ", err)
	}
	db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucket([]byte(tlsSessionBucket))
		if err != nil {
			log.Fatal("Failed to create bucket: ", err)
			return err
		}
		return nil
	})

	server := &server{
		TLSConf: &tls.Config{
			Certificates:       []tls.Certificate{cert},
			InsecureSkipVerify: true, //TODO dev only
			ClientSessionCache: sessionCache{DB: db},
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
		if _, err := apiService.WriteWithSize(conn, data); err != nil {
			log.Printf("Request error %v", err)
			http.Error(w, "Internal application error", http.StatusInternalServerError)
			return
		}
		var response []*orgPB.Organization
		for {
			buf, _, err := apiService.ReadWithSize(conn)
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

type sessionCache struct {
	DB *bolt.DB
}

func (c sessionCache) Get(sessionKey string) (session *tls.ClientSessionState, ok bool) {
	c.DB.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(tlsSessionBucket))
		v := b.Get([]byte(sessionKey))
		if v != nil {
			session = &tls.ClientSessionState{}
			err := json.Unmarshal(v, session)
			if err != nil {
				log.Print("Error retrieving sessionState from cache: ", err)
				ok = false
			} else {
				ok = true
			}
		} else {
			ok = false
		}
		return nil
	})
	if ok {
		log.Println("Session cache hit")
	} else {
		log.Println("Session cache miss")
	}
	return
}

func (c sessionCache) Put(sessionKey string, cs *tls.ClientSessionState) {
	json, err := json.Marshal(cs)
	if err != nil {
		log.Print("Error saving sessionState to cache: ", err)
		return
	}
	c.DB.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(tlsSessionBucket))
		err := b.Put([]byte(sessionKey), json)
		if err != nil {
			log.Print("Error saving sessionState to cache: ", err)
		}
		return err
	})
}

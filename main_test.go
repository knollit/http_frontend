package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type endpointMock struct {
	buf bytes.Buffer
}

func (e *endpointMock) Read(p []byte) (n int, err error) {
	return e.buf.Read(p)
}

func (e *endpointMock) Write(p []byte) (n int, err error) {
	return len(p), nil
}

func (e *endpointMock) Close() error {
	return nil
}

func (e *endpointMock) LocalAddr() net.Addr {
	addrs, _ := net.InterfaceAddrs()
	return addrs[0]
}

func (e *endpointMock) RemoteAddr() net.Addr {
	addrs, _ := net.InterfaceAddrs()
	return addrs[0]
}

func (e *endpointMock) SetDeadline(t time.Time) error {
	return nil
}

func (e *endpointMock) SetReadDeadline(t time.Time) error {
	return nil
}

func (e *endpointMock) SetWriteDeadline(t time.Time) error {
	return nil
}

func TestGETEndpoint(t *testing.T) {
	//TODO setup

	e := &endpointMock{}
	s := &server{
		getOrgSvcConn: func() (net.Conn, error) {
			return e, nil
		},
	}
	ts := httptest.NewServer(s.rootHandler())
	defer ts.Close()

	res, err := http.Get(ts.URL + "/testOrg/testEndpoint/v1")
	if err != nil {
		t.Fatal("GET error: ", err)
	}
	if expectedStatus := 200; res.StatusCode != expectedStatus {
		t.Fatalf("Expected %v status, got %v", expectedStatus, res.StatusCode)
	}
	endpointData, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Fatal("Error reading response body: ", err)
	}
	var endpoint map[string]string
	if err := json.Unmarshal(endpointData, &endpoint); err != nil {
		t.Fatal("Error unmarshalling response data: ", err)
	}
	if len(endpoint["URL"]) == 0 {
		t.Fatalf("Expected JSON with a URL property. Got: %v", string(endpointData))
	}
}

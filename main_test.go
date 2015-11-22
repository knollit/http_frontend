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

	"github.com/google/flatbuffers/go"
	"github.com/mikeraimondi/knollit/common"
	"github.com/mikeraimondi/knollit/http_frontend/organizations"
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

func TestGETOrgs(t *testing.T) {
	// Start test server
	e := &endpointMock{}
	s := newServer()
	s.getOrgSvcConn = func() (net.Conn, error) {
		return e, nil
	}
	ts := httptest.NewServer(s.rootHandler())
	defer ts.Close()

	// Prepare response from org svc
	b := flatbuffers.NewBuilder(0)
	const orgName = "testOrg"
	namePosition := b.CreateByteString([]byte(orgName))
	organizations.OrganizationStart(b)
	organizations.OrganizationAddName(b, namePosition)
	orgPosition := organizations.OrganizationEnd(b)
	b.Finish(orgPosition)
	common.WriteWithSize(&e.buf, b.Bytes[b.Head():])

	// Do test
	res, err := http.Get(ts.URL + "/organizations")
	if err != nil {
		t.Fatal("GET error: ", err)
	}
	if expectedStatus := 200; res.StatusCode != expectedStatus {
		t.Fatalf("Expected %v status, got %v", expectedStatus, res.StatusCode)
	}
	orgData, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Fatal("Error reading response body: ", err)
	}
	var organizations map[string]string
	if err := json.Unmarshal(orgData, &organizations); err != nil {
		t.Fatal("Error unmarshalling response data: ", err)
	}
	if len(organizations["name"]) == 0 {
		t.Fatalf("Expected JSON with a URL property. Got: %v", string(orgData))
	}
	if organizations["name"] != orgName {
		t.Fatalf("Expected %v for name. Got %v", orgName, organizations["name"])
	}
}

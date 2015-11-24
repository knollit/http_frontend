package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/flatbuffers/go"
	"github.com/mikeraimondi/knollit/common"
	"github.com/mikeraimondi/knollit/endpoints/endpoints"
	"github.com/mikeraimondi/knollit/http_frontend/organizations"
)

type serviceStub struct {
	buf      bytes.Buffer
	writeBuf bytes.Buffer
}

func (e *serviceStub) Read(p []byte) (n int, err error) {
	return e.buf.Read(p)
}

func (e *serviceStub) Write(p []byte) (n int, err error) {
	return e.writeBuf.Write(p)
}

func (e *serviceStub) Close() error {
	return nil
}

func (e *serviceStub) LocalAddr() net.Addr {
	addrs, _ := net.InterfaceAddrs()
	return addrs[0]
}

func (e *serviceStub) RemoteAddr() net.Addr {
	addrs, _ := net.InterfaceAddrs()
	return addrs[0]
}

func (e *serviceStub) SetDeadline(t time.Time) error {
	return nil
}

func (e *serviceStub) SetReadDeadline(t time.Time) error {
	return nil
}

func (e *serviceStub) SetWriteDeadline(t time.Time) error {
	return nil
}

func TestGETOrgs(t *testing.T) {
	// Start test server
	orgSvcStub := &serviceStub{}
	s := newServer()
	s.getOrgSvcConn = func() (net.Conn, error) {
		return orgSvcStub, nil
	}
	ts := httptest.NewServer(s.handler())
	defer ts.Close()

	// Prepare response from org svc
	b := flatbuffers.NewBuilder(0)
	const orgName = "testOrg"
	namePosition := b.CreateByteString([]byte(orgName))
	organizations.OrganizationStart(b)
	organizations.OrganizationAddName(b, namePosition)
	orgPosition := organizations.OrganizationEnd(b)
	b.Finish(orgPosition)
	common.WriteWithSize(&orgSvcStub.buf, b.Bytes[b.Head():])

	// Perform test
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
	var organizations []map[string]string
	if err := json.Unmarshal(orgData, &organizations); err != nil {
		t.Fatal("Error unmarshalling response data: ", err)
	}
	if len(organizations) != 1 {
		t.Fatalf("Expected JSON array with 1 element. Got: %v", organizations)
	}
	org := organizations[0]
	if len(org["name"]) == 0 {
		t.Fatalf("Expected JSON with a URL property. Got: %v", string(orgData))
	}
	if org["name"] != orgName {
		t.Fatalf("Expected %v for name. Got %v", orgName, org["name"])
	}
}

func TestGETEndpoint(t *testing.T) {
	// Start test server
	endpointSvc := &serviceStub{}
	s := newServer()
	s.getEndpointSvcConn = func() (net.Conn, error) {
		return endpointSvc, nil
	}
	ts := httptest.NewServer(s.handler())
	defer ts.Close()

	// Prepare response from endpoint service
	endpoint := endpoint{
		ID:           "5ff0fcbd-8b51-11e5-a171-df11d9bd7d62",
		Organization: "testOrg",
		URL:          "http://test.com",
	}
	b := flatbuffers.NewBuilder(0)
	common.WriteWithSize(&endpointSvc.buf, endpoint.toFlatBufferBytes(b))

	// Perform test
	res, err := http.Get(fmt.Sprintf("%v/organizations/%v/endpoints/%v", ts.URL, endpoint.Organization, endpoint.ID))
	if err != nil {
		t.Fatal("GET error: ", err)
	}
	if expectedStatus := 200; res.StatusCode != expectedStatus {
		t.Fatalf("Expected %v status, got %v", expectedStatus, res.StatusCode)
	}
	buf, _, err := common.ReadWithSize(&endpointSvc.writeBuf)
	if err != nil {
		t.Fatal(err)
	}
	endpointMsg := endpoints.GetRootAsEndpoint(buf, 0)
	if id := string(endpointMsg.Id()); id != endpoint.ID {
		t.Fatalf("Expected %v in endpoint request, got %v", endpoint.ID, id)
	}
	endpointData, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Fatal("Error reading response body: ", err)
	}
	var endpointJSON map[string]string
	if err := json.Unmarshal(endpointData, &endpointJSON); err != nil {
		t.Fatal("Error unmarshalling response data: ", err)
	}
	if endpointJSON["URL"] != endpoint.URL {
		t.Fatalf("Expected %v for URL. Got %v", endpoint.URL, endpointJSON["URL"])
	}
}

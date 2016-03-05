package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/google/flatbuffers/go"
	"github.com/knollit/http_frontend/endpoints"
	"github.com/knollit/http_frontend/organizations"
	"github.com/mikeraimondi/prefixedio"
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
	t.Parallel()
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
	prefixedio.WriteBytes(&orgSvcStub.buf, b.Bytes[b.Head():])

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
	t.Parallel()

	// Start test server
	endpointSvc := &serviceStub{}
	organizationSvc := &serviceStub{}
	s := newServer()
	s.getEndpointSvcConn = func() (net.Conn, error) {
		return endpointSvc, nil
	}
	s.getOrgSvcConn = func() (net.Conn, error) {
		return organizationSvc, nil
	}
	ts := httptest.NewServer(s.handler())
	defer ts.Close()

	// Prepare response from organization service
	org := organization{
		ID:   "5ff0fcbe-8b51-11e5-a171-df11d9bd7d62",
		Name: "testOrg",
	}
	b := flatbuffers.NewBuilder(0)
	prefixedio.WriteBytes(&organizationSvc.buf, org.toFlatBufferBytes(b))

	// Prepare response from endpoint service
	endpoint := endpoint{
		ID:  "5ff0fcbd-8b51-11e5-a171-df11d9bd7d62",
		URL: "http://test.com",
	}
	b.Reset()
	prefixedio.WriteBytes(&endpointSvc.buf, endpoint.toFlatBufferBytes(b))

	// Make test request
	res, err := http.Get(fmt.Sprintf("%v/organizations/%v/endpoints/%v", ts.URL, org.Name, endpoint.ID))
	if err != nil {
		t.Fatal("GET error: ", err)
	}

	// Test response status
	if expectedStatus := 200; res.StatusCode != expectedStatus {
		t.Fatalf("Expected %v status, got %v", expectedStatus, res.StatusCode)
	}

	// Test organization service is contacted
	var buf prefixedio.Buffer
	if _, err = buf.ReadFrom(&organizationSvc.writeBuf); err != nil {
		t.Fatal(err)
	}
	organizationMsg := organizations.GetRootAsOrganization(buf.Bytes(), 0)
	if name := string(organizationMsg.Name()); name != org.Name {
		t.Fatalf("Expected %v in organization request, got %v", org.Name, name)
	}
	if organizationMsg.Action() != organizations.ActionRead {
		t.Fatalf("Expected %v for action in request, got %v", organizations.ActionRead, organizationMsg.Action())
	}

	// Test endpoint service is contacted
	if _, err = buf.ReadFrom(&endpointSvc.writeBuf); err != nil {
		t.Fatal(err)
	}
	endpointMsg := endpoints.GetRootAsEndpoint(buf.Bytes(), 0)
	if id := string(endpointMsg.Id()); id != endpoint.ID {
		t.Fatalf("Expected %v in endpoint request, got %v", endpoint.ID, id)
	}

	// Test response
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

func TestGETEndpointNoResult(t *testing.T) {
	t.Parallel()

	// Start test server
	endpointSvc := &serviceStub{}
	organizationSvc := &serviceStub{}
	s := newServer()
	s.getEndpointSvcConn = func() (net.Conn, error) {
		return endpointSvc, nil
	}
	s.getOrgSvcConn = func() (net.Conn, error) {
		return organizationSvc, nil
	}
	ts := httptest.NewServer(s.handler())
	defer ts.Close()

	// Prepare response from organization service
	org := organization{
		ID:   "5ff0fcbe-8b51-11e5-a171-df11d9bd7d62",
		Name: "testOrg",
	}
	b := flatbuffers.NewBuilder(0)
	prefixedio.WriteBytes(&organizationSvc.buf, org.toFlatBufferBytes(b))

	// Prepare response from endpoint service
	endpoint := endpoint{
		ID:  "5ff0fcbd-8b51-11e5-a171-df11d9bd7d62",
		err: errors.New(notFoundErrMsg),
	}
	b.Reset()
	prefixedio.WriteBytes(&endpointSvc.buf, endpoint.toFlatBufferBytes(b))

	// Make test request
	res, err := http.Get(fmt.Sprintf("%v/organizations/%v/endpoints/%v", ts.URL, org.Name, endpoint.ID))
	if err != nil {
		t.Fatal("GET error: ", err)
	}

	// Test response status
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("status code does not match. expected: %v. actual: %v.\n", http.StatusNotFound, res.StatusCode)
	}

	// Test response
	endpointData, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Fatal("Error reading response body: ", err)
	}
	var endpointJSON map[string]string
	if err := json.Unmarshal(endpointData, &endpointJSON); err != nil {
		t.Fatal("Error unmarshalling response data: ", err)
	}
	if endpointJSON["ID"] != endpoint.ID {
		t.Fatalf("Expected %v for ID. Got %v", endpoint.ID, endpointJSON["ID"])
	}
}

func TestPOSTEndpoint(t *testing.T) {
	t.Parallel()

	// Start test server
	endpointSvc := &serviceStub{}
	organizationSvc := &serviceStub{}
	s := newServer()
	s.getEndpointSvcConn = func() (net.Conn, error) {
		return endpointSvc, nil
	}
	s.getOrgSvcConn = func() (net.Conn, error) {
		return organizationSvc, nil
	}
	ts := httptest.NewServer(s.handler())
	defer ts.Close()

	// Prepare response from organization service
	org := organization{
		Name: "testOrg",
		ID:   "5ff0fcbe-8b51-11e5-a171-df11d9bd7d62",
	}
	b := flatbuffers.NewBuilder(0)
	prefixedio.WriteBytes(&organizationSvc.buf, org.toFlatBufferBytes(b))

	// Prepare response from endpoint service
	endpoint := endpoint{
		ID:  "5ff0fcbd-8b51-11e5-a171-df11d9bd7d62",
		URL: "http://test.com",
	}
	b.Reset()
	prefixedio.WriteBytes(&endpointSvc.buf, endpoint.toFlatBufferBytes(b))

	// Make test request
	res, err := http.PostForm(fmt.Sprintf("%v/organizations/%v/endpoints", ts.URL, org.Name), url.Values{"url": {endpoint.URL}})
	if err != nil {
		t.Fatal("POST error: ", err)
	}

	// Test response status
	if expectedStatus := http.StatusCreated; res.StatusCode != expectedStatus {
		t.Fatalf("status code does not match. expected: %v. actual: %v\n", expectedStatus, res.StatusCode)
	}

	// Test endpoint service is contacted
	var buf prefixedio.Buffer
	if _, err = buf.ReadFrom(&endpointSvc.writeBuf); err != nil {
		t.Fatal(err)
	}
	endpointMsg := endpoints.GetRootAsEndpoint(buf.Bytes(), 0)
	if action := endpointMsg.Action(); action != endpoints.ActionNew {
		t.Fatalf("action does not match. expected: %v. actual: %v\n", endpoints.ActionNew, action)
	}

	// Test response
	endpointData, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Fatal("error reading response body: ", err)
	}
	var endpointJSON map[string]string
	if err := json.Unmarshal(endpointData, &endpointJSON); err != nil {
		t.Fatal("error unmarshalling response data: ", err)
	}
	if endpointJSON["URL"] != endpoint.URL {
		t.Fatalf("JSON URL does not match. expected: %v. actual: %v\n", endpoint.URL, endpointJSON["URL"])
	}
}

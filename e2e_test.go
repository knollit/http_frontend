package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"testing"

	"github.com/mikeraimondi/go_compose_testing"
)

// TODO should be inferred from go_compose
const port = ":6080"

func assertGet(t *testing.T, url string, expected interface{}) {
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	buf := &bytes.Buffer{}
	json.NewEncoder(buf).Encode(expected)
	if res, err := ioutil.ReadAll(resp.Body); string(res) != string(buf.Bytes()) {
		t.Fatalf("response from server does not match. Expected: %s. Actual: %s.\n", bytes.TrimSpace(buf.Bytes()), bytes.TrimSpace(res))
	} else if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
}

func TestOrganizationIndexE2E(t *testing.T) {
	composeTesting.Run(t, port, func(ip []byte) {
		orgURL := fmt.Sprintf("http://%s%v/organizations", ip, port)

		assertGet(t, orgURL, []organization{})

		orgName := "testOrg"
		resp, err := http.PostForm(orgURL, url.Values{"name": {orgName}})
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("status code does not match. expected: %v. actual: %v\n", http.StatusCreated, resp.StatusCode)
		}
		resp.Body.Close()

		assertGet(t, orgURL, []organization{organization{
			Name: orgName,
		},
		})
	})
}

func TestEndpointPostE2E(t *testing.T) {
	// TODO
}

func TestEndpointReadE2E(t *testing.T) {
	composeTesting.Run(t, port, func(ip []byte) {
		// setup: create organization
		orgURL := fmt.Sprintf("http://%s%v/organizations", ip, port)
		orgName := "testOrg"
		resp, err := http.PostForm(orgURL, url.Values{"name": {orgName}})
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()

		// setup: create endpoint
		endpointURL := fmt.Sprintf("http://%s%v/organizations/%v/endpoints", ip, port, orgName)
		resp, err = http.PostForm(endpointURL, url.Values{"url": {"some url"}})
		if err != nil {
			t.Fatal(err)
		}
		data, _ := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		newEndpoint := endpoint{}
		json.Unmarshal(data, &newEndpoint)
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("status code does not match. expected: %v. actual: %v\n", http.StatusCreated, resp.StatusCode)
		}

		// test: endpoint GET
		assertGet(t, endpointURL+"/"+newEndpoint.ID, newEndpoint)
	})
}

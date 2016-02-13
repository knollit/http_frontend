package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"testing"

	"github.com/mikeraimondi/go_compose"
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
		t.Fatalf("Response from server does not match. Expected: %s. Actual: %s.\n", bytes.TrimSpace(buf.Bytes()), bytes.TrimSpace(res))
	} else if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
}

func TestOrganizationIndexE2E(t *testing.T) {
	compose.RunTest(t, port, func(ip []byte) {
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

func TestEndpointReadE2E(t *testing.T) {
	compose.RunTest(t, port, func(ip []byte) {
		orgURL := fmt.Sprintf("http://%s%v/organizations", ip, port)
		orgName := "testOrg"
		resp, err := http.PostForm(orgURL, url.Values{"name": {orgName}})
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()

		endpointURL := fmt.Sprintf("http://%s%v/testOrg/endpoints", ip, port)
		resp, _ = http.Head(endpointURL + "/foobar")
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("status code does not match. expected: %v. actual: %v.\n", http.StatusNotFound, resp.StatusCode)
		}
		resp.Body.Close()

		resp, err = http.PostForm(endpointURL, url.Values{"url": {"some url"}})
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("status code does not match. expected: %v. actual: %v\n", http.StatusCreated, resp.StatusCode)
		}
		resp.Body.Close()

		assertGet(t, endpointURL, endpoint{
			URL: "some url",
		})
	})
}

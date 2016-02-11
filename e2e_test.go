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
		t.Fatalf("Response from server does not match. Expected: %s. Actual: %s.\n", buf.Bytes(), res)
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
		resp.Body.Close()

		assertGet(t, orgURL, []organization{organization{
			Name: orgName,
		},
		})
	})
}

func TestEndpointReadE2E(t *testing.T) {
	t.SkipNow()
	compose.RunTest(t, port, func(ip []byte) {
		endpointURL := fmt.Sprintf("http://%s%v/endpoint/", ip, port)

		assertGet(t, endpointURL, []organization{})

		orgName := "testOrg"
		resp, err := http.PostForm(endpointURL, url.Values{"name": {orgName}})
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()

		assertGet(t, endpointURL, []organization{organization{
			Name: orgName,
		},
		})
	})
}

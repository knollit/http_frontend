package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/mikeraimondi/gompose_testing"
)

const port = ":6080"

func TestMain(m *testing.M) {
	gomposeTesting.RegisterBeforeCallback(func(t *testing.T) {
		if err := exec.Command("make").Run(); err != nil {
			t.Fatal("make failed: ", err)
		}
	})

	gomposeTesting.RegisterBeforeEachCallback(func(t *testing.T, ip []byte) {
		// poll until server is healthy
		start := time.Now()
		for func() bool {
			resp, err := http.Head(fmt.Sprintf("http://%s%v/health_check", ip, port))
			if err == nil && resp.StatusCode == 204 {
				return false
			}
			return true
		}() {
			if time.Now().Sub(start) > time.Second*30 {
				t.Fatal("timed out waiting for server to start.")
			}
			time.Sleep(time.Millisecond * 250)
		}
	})

	flag.Parse()
	os.Exit(m.Run())
}

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
	gomposeTesting.Run(t, func(ip []byte) {
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
	gomposeTesting.Run(t, func(ip []byte) {
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

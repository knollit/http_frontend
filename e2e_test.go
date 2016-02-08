package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"testing"
	"time"
)

const port = ":6080"

func e2e(t *testing.T, testFunc func([]byte)) {
	if testing.Short() {
		t.Skip("skipping end-to-end test in short mode.")
	}
	var ip []byte
	dkm, err := exec.Command("docker-machine", "active").Output()
	if err == nil { // active Docker Machine detected, use it
		byteIP, err := exec.Command("docker-machine", "ip", string(bytes.TrimSpace(dkm))).Output()
		if err != nil {
			t.Fatal(err)
		}
		ip = bytes.TrimSpace(byteIP)
	} else { // no active docker machine, assume Docker is running natively
		ip = []byte("127.0.0.1")
	}

	// TODO run only once over all end-to-end tests
	if err := exec.Command("./build.sh").Run(); err != nil {
		t.Fatal("Build failed: ", err)
	}
	if err := exec.Command("docker-compose", "up", "-d").Run(); err != nil {
		t.Fatal("Docker compose failed to start: ", err)
	}
	defer func() {
		if err := exec.Command("docker-compose", "stop").Run(); err != nil {
			t.Fatal(err)
		}
		// TODO reset the DBs with fewer side effects
		if err := exec.Command("docker-compose", "rm", "-f").Run(); err != nil {
			t.Fatal(err)
		}
	}()
	// TODO timestamps
	file, _ := os.Create("test.log")
	cmd := exec.Command("docker-compose", "logs", "--no-color")
	cmd.Stdout = file
	cmd.Stderr = file
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cmd.Process.Signal(os.Interrupt)
		cmd.Wait()
	}()

	// Poll until server is healthy
	attempts := 0
	startTS := time.Now()
	for {
		if waited := time.Second * 30; time.Now().Sub(startTS) > waited {
			t.Fatalf("Timed out waiting for server to start. Waited %v.", waited)
		}
		res, err := http.Head(fmt.Sprintf("http://%s%v/health_check", ip, port))
		if err == nil && res.StatusCode == 204 {
			break
		} else {
			attempts++
			time.Sleep(time.Millisecond * 250)
		}
	}
	testFunc(ip)
}

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
	e2e(t, func(ip []byte) {
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

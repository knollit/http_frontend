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

type e2eContext struct {
	built   bool
	ip      []byte
	logFile *os.File
	testNum int
}

var context e2eContext

func e2e(t *testing.T, testFunc func([]byte)) {
	if testing.Short() {
		t.Skip("skipping end-to-end test in short mode.")
	}

	// build project if unbuilt
	if !context.built {
		if err := exec.Command("./build.sh").Run(); err != nil {
			t.Fatal("build failed: ", err)
		}
		context.built = true
	}

	// get Docker IP and cache it
	if len(context.ip) == 0 {
		dkm, err := exec.Command("docker-machine", "active").Output()
		if err == nil { // active Docker Machine detected, use it
			byteIP, err := exec.Command("docker-machine", "ip", string(bytes.TrimSpace(dkm))).Output()
			if err != nil {
				t.Fatal(err)
			}
			context.ip = bytes.TrimSpace(byteIP)
		} else { // no active docker machine, assume Docker is running natively
			context.ip = []byte("127.0.0.1")
		}
	}

	// bring up Compose
	if err := exec.Command("docker-compose", "up", "-d").Run(); err != nil {
		t.Fatal("Docker compose failed to start: ", err)
	}
	defer func() {
		if err := exec.Command("docker-compose", "down").Run(); err != nil {
			t.Fatal(err)
		}
	}()

	// log Compose output
	// TODO timestamps
	if context.logFile == nil {
		context.logFile, _ = os.Create("test.log")
		cmd := exec.Command("docker-compose", "logs", "--no-color")
		cmd.Stdout = context.logFile
		cmd.Stderr = context.logFile
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
	}
	context.testNum++
	context.logFile.WriteString(fmt.Sprintf("--- test %v start\n", context.testNum))
	defer context.logFile.WriteString(fmt.Sprintf("--- test %v end\n", context.testNum))

	// poll until server is healthy
	start := time.Now()
	for func() bool {
		resp, err := http.Head(fmt.Sprintf("http://%s%v/health_check", context.ip, port))
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

	// Run test
	testFunc(context.ip)
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

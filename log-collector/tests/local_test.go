package tests

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"testing"
	"time"
)

func TestLocalOutput(t *testing.T) {
	empty := func() error {
		return fmt.Errorf("format string")
	}
	retry(10, time.Second*10, empty)

	t.Run("Pod", testPod)
	t.Run("Service", testService)
}

func testPod(t *testing.T) {
	dir, err := ioutil.TempDir("", "pod")
	if err != nil {
		log.Fatal(err)
	}
	if err := cr.WithLocalOuput(dir); err != nil {
		t.Fatalf("error init local output %v", err)
	}

	results, err := cr.CollectPodLogs("kube-apiserver")
	if err != nil {
		t.Fatalf("error in collecting %v", err)
	}

	if len(results) == 0 {
		t.Fatalf("alteast one container exists but not found")
	}
	t.Log(results)

	os.RemoveAll(dir)
}

func testService(t *testing.T) {
	dir, err := ioutil.TempDir("", "pod")
	if err != nil {
		log.Fatal(err)
	}
	if err := cr.WithLocalOuput(dir); err != nil {
		t.Fatalf("error init local output %v", err)
	}

	results, err := cr.CollectServiceLogs("kubelet")
	if err != nil {
		t.Fatalf("error in collecting %v", err)
	}

	if len(results) == 0 {
		t.Fatalf("alteast one service exists but not found")
	}
	t.Log(results)

	os.RemoveAll(dir)
}

package tests

import (
	"testing"

	"k8s.io/client-go/pkg/api/v1"

	"time"

	"github.com/coreos/ktestutil/nginx"
	"github.com/coreos/ktestutil/utils"
)

func TestSucceededPing(t *testing.T) {
	nginx, err := nginx.NewNginx(client, namespace)
	if err != nil {
		t.Fatal(err)
	}
	defer nginx.Delete()

	if err := utils.Retry(10, 2*time.Second, func() error {
		return nginx.Ping(v1.PodSucceeded)
	}); err != nil {
		t.Fatal(err)
	}
}
func TestFailedPing(t *testing.T) {
	nginx, err := nginx.NewNginx(client, namespace)
	if err != nil {
		t.Fatal(err)
	}
	nginx.Delete()

	if err := utils.Retry(10, 5*time.Second, func() error {
		return nginx.Ping(v1.PodFailed)
	}); err != nil {
		t.Fatal(err)
	}
}

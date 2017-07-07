package tests

import (
	"testing"
	"time"

	"github.com/coreos/ktestutil/nginx"
	"github.com/coreos/ktestutil/utils"
)

func TestReachable(t *testing.T) {
	nginx, err := nginx.NewNginx(client, namespace)
	if err != nil {
		t.Fatal(err)
	}
	defer nginx.Delete()

	if err := utils.Retry(10, 2*time.Second, func() error {
		return nginx.IsReachable()
	}); err != nil {
		t.Fatal(err)
	}
}
func TestUnReachable(t *testing.T) {
	nginx, err := nginx.NewNginx(client, namespace)
	if err != nil {
		t.Fatal(err)
	}
	nginx.Delete()

	if err := utils.Retry(10, 5*time.Second, func() error {
		return nginx.IsUnReachable()
	}); err != nil {
		t.Fatal(err)
	}
}

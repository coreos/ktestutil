package tests

import (
	"fmt"
	"testing"
	"time"

	"github.com/coreos/ktestutil/testworkload"
	"github.com/coreos/ktestutil/utils"
)

func TestReachable(t *testing.T) {
	n, err := testworkload.NewNginx(client, namespace)
	if err != nil {
		t.Fatal(err)
	}
	defer n.Delete()

	if err := utils.Retry(10, 2*time.Second, func() error {
		return n.IsReachable()
	}); err != nil {
		t.Fatal(err)
	}

	if err := utils.Retry(10, 2*time.Second, func() error {
		if err := n.IsUnReachable(); err == nil {
			return fmt.Errorf("error should be not nil")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
func TestUnReachable(t *testing.T) {
	n, err := testworkload.NewNginx(client, namespace)
	if err != nil {
		t.Fatal(err)
	}
	n.Delete()

	if err := utils.Retry(10, 5*time.Second, func() error {
		return n.IsUnReachable()
	}); err != nil {
		t.Fatal(err)
	}

	if err := utils.Retry(10, 2*time.Second, func() error {
		if err := n.IsReachable(); err == nil {
			return fmt.Errorf("error should be not nil")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

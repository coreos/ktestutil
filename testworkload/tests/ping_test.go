package tests

import (
	"testing"
	"time"

	"fmt"

	"github.com/coreos/ktestutil/testworkload"
	"github.com/coreos/ktestutil/utils"
)

func TestReachable(t *testing.T) {
	tw, err := testworkload.New(client, namespace)
	if err != nil {
		t.Fatal(err)
	}
	defer tw.Delete()

	if err := utils.Retry(10, 2*time.Second, func() error {
		return tw.IsReachable()
	}); err != nil {
		t.Fatal(err)
	}

	if err := utils.Retry(10, 2*time.Second, func() error {
		if err := tw.IsUnReachable(); err == nil {
			return fmt.Errorf("error should be not nil")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
func TestUnReachable(t *testing.T) {
	tw, err := testworkload.New(client, namespace)
	if err != nil {
		t.Fatal(err)
	}
	tw.Delete()

	if err := utils.Retry(10, 5*time.Second, func() error {
		return tw.IsUnReachable()
	}); err != nil {
		t.Fatal(err)
	}

	if err := utils.Retry(10, 2*time.Second, func() error {
		if err := tw.IsReachable(); err == nil {
			return fmt.Errorf("error should be not nil")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

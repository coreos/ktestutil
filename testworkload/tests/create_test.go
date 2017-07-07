package tests

import (
	"fmt"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/coreos/ktestutil/testworkload"
)

func TestCreate(t *testing.T) {
	tw, err := testworkload.New(client, namespace)
	if err != nil {
		t.Fatal(err)
	}
	defer tw.Delete()

	t.Run("CheckDeployExists", checkDeployExists(tw))
	t.Run("CheckRunningPodsCount", checkRunningPodsCount(tw))
	t.Run("CheckSerivceExists", checkSerivceExists(tw))
}

func checkDeployExists(tw *testworkload.TestWorkload) func(*testing.T) {
	return func(t *testing.T) {
		_, err := client.ExtensionsV1beta1().Deployments(namespace).Get(tw.Name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				t.Fatalf("not found deployment %s: %v", tw.Name, err)
			}
			t.Fatalf("error finding deployment %s: %v", tw.Name, err)
		}
	}
}

func checkRunningPodsCount(tw *testworkload.TestWorkload) func(*testing.T) {
	return func(t *testing.T) {
		d, err := client.ExtensionsV1beta1().Deployments(namespace).Get(tw.Name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				t.Fatalf("not found deployment %s: %v", tw.Name, err)
			}
			t.Fatalf("error finding deployment %s: %v", tw.Name, err)
		}

		pl, err := client.CoreV1().Pods(namespace).List(metav1.ListOptions{LabelSelector: fmt.Sprintf("app=%s", tw.Name)})
		if err != nil {
			t.Fatalf("error getting pods for deployment %s: %v", tw.Name, err)
		}

		if int(d.Status.UpdatedReplicas) != len(pl.Items) {
			t.Fatalf("needed pods: %d found: %d", d.Status.UpdatedReplicas, len(pl.Items))
		}
	}
}

func checkSerivceExists(tw *testworkload.TestWorkload) func(*testing.T) {
	return func(t *testing.T) {
		_, err := client.CoreV1().Services(namespace).Get(tw.Name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				t.Fatalf("not found service %s: %v", tw.Name, err)
			}
			t.Fatalf("error finding service %s: %v", tw.Name, err)
		}
	}
}

package tests

import (
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"fmt"

	"github.com/coreos/ktestutil/nginx"
)

func TestCreate(t *testing.T) {
	nginx, err := nginx.NewNginx(client, namespace)
	if err != nil {
		t.Fatal(err)
	}
	defer nginx.Delete()

	t.Run("CheckDeployExists", checkDeployExists(nginx))
	t.Run("CheckRunningPodsCount", checkRunningPodsCount(nginx))
	t.Run("CheckSerivceExists", checkSerivceExists(nginx))
}

func checkDeployExists(nginx *nginx.Nginx) func(*testing.T) {
	return func(t *testing.T) {
		_, err := client.ExtensionsV1beta1().Deployments(namespace).Get(nginx.Name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				t.Fatalf("not found deployment %s: %v", nginx.Name, err)
			}
			t.Fatalf("error finding deployment %s: %v", nginx.Name, err)
		}
	}
}

func checkRunningPodsCount(nginx *nginx.Nginx) func(*testing.T) {
	return func(t *testing.T) {
		d, err := client.ExtensionsV1beta1().Deployments(namespace).Get(nginx.Name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				t.Fatalf("not found deployment %s: %v", nginx.Name, err)
			}
			t.Fatalf("error finding deployment %s: %v", nginx.Name, err)
		}

		pl, err := client.CoreV1().Pods(namespace).List(metav1.ListOptions{LabelSelector: fmt.Sprintf("app=%s", nginx.Name)})
		if err != nil {
			t.Fatalf("error getting pods for deployment %s: %v", nginx.Name, err)
		}

		if int(d.Status.UpdatedReplicas) != len(pl.Items) {
			t.Fatalf("needed pods: %d found: %d", d.Status.UpdatedReplicas, len(pl.Items))
		}
	}
}

func checkSerivceExists(nginx *nginx.Nginx) func(*testing.T) {
	return func(t *testing.T) {
		_, err := client.CoreV1().Services(namespace).Get(nginx.SVCName, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				t.Fatalf("not found service %s: %v", nginx.SVCName, err)
			}
			t.Fatalf("error finding service %s: %v", nginx.SVCName, err)
		}
	}
}

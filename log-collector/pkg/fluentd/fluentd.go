package fluentd

import (
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
)

const (
	fluentMasterNodeAnnotation = "log-collector.github.com/fluentd-master"
)

// CreateAssets creates fluent master deployment and fluent worker daemonset.
// all fluent workers stream their local logs to the fluent master.
// fluent master writes all the logs to the disk at `/var/log/log-collector/`
// fluent master is pinned to a master node with annotation `log-collector.github.com/fluentd-master`
//
// CreateAssets waits for fluent master pod and atleast one fluent worker pod to be running
func CreateAssets(client kubernetes.Interface, namespace string) error {
	// tag one of the masters
	nl, err := client.CoreV1().Nodes().List(metav1.ListOptions{LabelSelector: "node-role.kubernetes.io/master="})
	if err != nil || len(nl.Items) == 0 {
		return fmt.Errorf("couldn't find master node to run fluent master %v", err)
	}

	n := nl.Items[0]
	n.ObjectMeta.Labels[fluentMasterNodeAnnotation] = ""

	if _, err := client.CoreV1().Nodes().Update(&n); err != nil {
		return fmt.Errorf("couldn't tag one of the master nodes %v", err)
	}

	// create assests
	if err := createMasterCfg(client, namespace); err != nil {
		return fmt.Errorf("error creating fluentd asset %v", err)
	}

	if err := createMasterDeploy(client, namespace); err != nil {
		return fmt.Errorf("error creating fluentd asset %v", err)
	}

	if err := createMasterSvc(client, namespace); err != nil {
		return fmt.Errorf("error creating fluentd asset %v", err)
	}

	if err := createWorkerCfg(client, namespace); err != nil {
		return fmt.Errorf("error creating fluentd asset %v", err)
	}

	if err := createWorkerDs(client, namespace); err != nil {
		return fmt.Errorf("error creating fluentd asset %v", err)
	}

	if err := retry(10, time.Second*10, waitPodReadyOnMaster(client, namespace)); err != nil {
		return err
	}

	if err := retry(10, time.Second*10, waitPodReadyOnWorker(client, namespace)); err != nil {
		return err
	}

	return nil
}

// DeleteAssets deletes all the fluentd assets.
// Also removes all node annotations
func DeleteAssets(client kubernetes.Interface, namespace string) error {
	if err := deleteMasterCfg(client, namespace); err != nil {
		return fmt.Errorf("error deleting fluentd asset %v", err)
	}

	if err := deleteMasterDeploy(client, namespace); err != nil {
		return fmt.Errorf("error deleting fluentd asset %v", err)
	}

	if err := deleteMasterSvc(client, namespace); err != nil {
		return fmt.Errorf("error deleting fluentd asset %v", err)
	}

	if err := deleteWorkerCfg(client, namespace); err != nil {
		return fmt.Errorf("error deleting fluentd asset %v", err)
	}

	if err := deleteWorkerDs(client, namespace); err != nil {
		return fmt.Errorf("error deleting fluentd asset %v", err)
	}

	labelSelc := fmt.Printf("%s=", fluentMasterNodeAnnotation)
	nl, err := client.CoreV1().Nodes().List(metav1.ListOptions{LabelSelector: labelSelc})
	if err != nil || len(nl.Items) == 0 {
		return fmt.Errorf("couldn't find master node running fluent master %v", err)
	}

	n := nl.Items[0]
	delete(n.ObjectMeta.Labels, fluentMasterNodeAnnotation)

	if _, err := client.CoreV1().Nodes().Update(&n); err != nil {
		return fmt.Errorf("couldn't delete annotation from the master node running fluent master %v", err)
	}

	return nil
}

// GetNodeAddressWithMaster return PublicAddressableIP of the master node that is running fluent master
func GetNodeAddressWithMaster(client kubernetes.Interface, namespace string) (string, error) {
	pl, err := client.CoreV1().Pods(namespace).List(metav1.ListOptions{LabelSelector: "k8s-app=fluentd-master,tier=control-plane"})
	if err != nil || len(pl.Items) == 0 {
		return "", fmt.Errorf("No pod from fluentd-master deployment found: %v", err)
	}

	p := &pl.Items[0]
	nodeName := p.Spec.NodeName

	n, err := client.CoreV1().Nodes().Get(nodeName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("No node found with name: %s", nodeName)
	}

	var host string
	for _, addr := range n.Status.Addresses {
		if addr.Type == v1.NodeExternalIP {
			host = addr.Address
		}

	}
	if host == "" {
		// try finding "LegacyHostIP"
		for _, addr := range n.Status.Addresses {
			if addr.Type == "LegacyHostIP" {
				host = addr.Address
			}
		}
	}

	if host == "" {
		return "", fmt.Errorf("could not get external node IP for node: %s", nodeName)
	}

	return host, nil
}

func retry(attempts int, delay time.Duration, f func() error) error {
	var err error

	for i := 0; i < attempts; i++ {
		err = f()
		if err == nil {
			break
		}

		if i < attempts-1 {
			time.Sleep(delay)
		}
	}

	return err
}

func waitPodReadyOnMaster(client kubernetes.Interface, namespace string) func() error {
	return func() error {
		l, err := client.CoreV1().Pods(namespace).List(metav1.ListOptions{LabelSelector: "k8s-app=fluentd-master"})
		if err != nil || len(l.Items) == 0 {
			return fmt.Errorf("pod not yet running: %v", err)
		}

		// take the first pod
		p := &l.Items[0]

		if p.Status.Phase != v1.PodRunning {
			return fmt.Errorf("pod not yet running: %v", p.Status.Phase)
		}
		return nil
	}
}

func waitPodReadyOnWorker(client kubernetes.Interface, namespace string) func() error {
	return func() error {
		l, err := client.CoreV1().Pods(namespace).List(metav1.ListOptions{LabelSelector: "k8s-app=fluentd-worker"})
		if err != nil || len(l.Items) == 0 {
			return fmt.Errorf("pod not yet running: %v", err)
		}

		// take the first pod
		p := &l.Items[0]

		if p.Status.Phase != v1.PodRunning {
			return fmt.Errorf("pod not yet running: %v", p.Status.Phase)
		}
		return nil
	}
}

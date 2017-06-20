package fluentd

import (
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
)

func CreateAssets(client kubernetes.Interface, namespace string) error {
	// tag one of the masters
	nl, err := client.CoreV1().Nodes().List(metav1.ListOptions{LabelSelector: "node-role.kubernetes.io/master="})
	if err != nil || len(nl.Items) == 0 {
		return fmt.Errorf("couldn't find master node %v", err)
	}

	n := nl.Items[0]
	n.ObjectMeta.Labels["log-collector.github.com/fluentd-master"] = ""

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

	if err := retry(10, time.Second*10, checkmaster(client, namespace)); err != nil {
		return err
	}

	if err := retry(10, time.Second*10, checkworker(client, namespace)); err != nil {
		return err
	}

	return nil
}

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

	nl, err := client.CoreV1().Nodes().List(metav1.ListOptions{LabelSelector: "log-collector.github.com/fluentd-master="})
	if err != nil || len(nl.Items) == 0 {
		return fmt.Errorf("couldn't find master node %v", err)
	}

	n := nl.Items[0]
	delete(n.ObjectMeta.Labels, "log-collector.github.com/fluentd-master")

	if _, err := client.CoreV1().Nodes().Update(&n); err != nil {
		return fmt.Errorf("couldn't tag one of the master nodes %v", err)
	}

	return nil
}

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

func checkmaster(client kubernetes.Interface, namespace string) func() error {
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

func checkworker(client kubernetes.Interface, namespace string) func() error {
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

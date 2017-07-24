package cluster

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/coreos/ktestutil/utils"

	"github.com/golang/glog"
	"golang.org/x/crypto/ssh"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
)

const (
	// We sleep 10 seconds to give some time for ssh command to cleanly finish.
	cmdReboot          = "nohup sh -c 'sleep 10 && sudo ifconfig eth0 down && sudo systemctl stop kubelet && sudo reboot' >/dev/null 2>&1 &"
	cmdRebootWithSleep = "nohup sh -c 'sleep 10 && sudo ifconfig eth0 down && sudo systemctl stop kubelet && sleep %ds && sudo reboot' >/dev/null 2>&1 &"

	cmdSystemUp = "sudo systemctl is-active kubelet"
)

// Cluster is a simple abstraction that stores cluster nodes.
// It allows rebooting the entire cluster / nodes.
type Cluster struct {
	// List of master nodes.
	Masters []*v1.Node
	// List of worker nodes.
	Workers []*v1.Node
	// MaxDisruption defines the max no. of nodes that will be rebooted in parallel.
	// Accepts int eg. '3' for no. of nodes, and string '30%' for percent.
	// Defaults to 100%.
	MaxDisruption intstr.IntOrString

	client    kubernetes.Interface
	sshClient *utils.SSHClient
	sshConfig *utils.SSHConfig
}

// New creates a new Cluster with the given options.
func New(client kubernetes.Interface, opts ...Options) (*Cluster, error) {
	cl := &Cluster{
		client:        client,
		sshConfig:     &utils.SSHConfig{},
		MaxDisruption: intstr.FromString("100%"),
	}
	for _, opt := range opts {
		opt(cl)
	}

	cl.sshClient = utils.MustNewSSHClient(cl.sshConfig)

	nodelist, err := client.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for i := range nodelist.Items {
		node := &nodelist.Items[i]
		switch {
		case utils.IsMaster(node):
			cl.Masters = append(cl.Masters, node)
		case utils.IsWorker(node):
			cl.Workers = append(cl.Workers, node)
		default:
			return nil, fmt.Errorf("node: %s is neither master nor worker", node.GetName())
		}
	}

	return cl, nil
}

// RebootAll reboots all the nodes that are accessible ie. have ExternalIP.
func (cl *Cluster) RebootAll(rebootDuration time.Duration) error {
	var hosts []string
	hosts = append(hosts, hostsFromNodes(cl.Masters)...)
	hosts = append(hosts, hostsFromNodes(cl.Workers)...)
	if len(hosts) < 1 {
		return fmt.Errorf("no nodes found that can be rebooted")
	}

	glog.V(4).Infof("will reboot nodes: %s", hosts)
	return cl.rebootHosts(hosts, rebootDuration)
}

// RebootMasters reboots all the master nodes that are accessible ie. have ExternalIP.
func (cl *Cluster) RebootMasters(rebootDuration time.Duration) error {
	hosts := hostsFromNodes(cl.Masters)
	if len(hosts) < 1 {
		return fmt.Errorf("no nodes found that can be rebooted")
	}

	glog.V(4).Infof("will reboot nodes: %s", hosts)
	return cl.rebootHosts(hosts, rebootDuration)
}

// RebootWorkers reboots all the worker nodes that are accessible ie. have ExternalIP.
func (cl *Cluster) RebootWorkers(rebootDuration time.Duration) error {
	hosts := hostsFromNodes(cl.Workers)
	if len(hosts) < 1 {
		return fmt.Errorf("no nodes found that can be rebooted")
	}

	glog.V(4).Infof("will reboot nodes: %s", hosts)
	return cl.rebootHosts(hosts, rebootDuration)
}

// RebootNode reboots a node addressable with `host`.
// Uses *Cluster sshClient.
func (cl *Cluster) RebootNode(host string, rebootDuration time.Duration) error {
	cmd := cmdReboot
	if rebootDuration.Seconds() > 0 {
		cmd = fmt.Sprintf(cmdRebootWithSleep, int(rebootDuration.Seconds()))
	}

	glog.V(4).Infof("node: %s rebooting", host)
	glog.V(4).Infof("node: %s executing cmd: '%s'", host, cmd)
	stdout, stderr, err := cl.sshClient.Exec(host, cmd)
	if _, ok := err.(*ssh.ExitMissingError); ok {
		// A terminated session is perfectly normal during reboot.
		err = nil
	}
	if err != nil {
		return fmt.Errorf("node: %s issuing command failed\nstdout:%s\nstderr:%s", host, stdout, stderr)
	}
	if err := cl.waitForDown(host); err != nil {
		return fmt.Errorf("node: %s didn't go down", host)
	}

	glog.V(4).Infof("node: %s rebooted waiting for node to come back up", host)
	if rebootDuration.Seconds() > 0 {
		<-time.After(rebootDuration)
	}
	if err := cl.waitForUp(host); err != nil {
		return fmt.Errorf("node: %s didn't come back up", host)
	}

	glog.V(4).Infof("node: %s reboot successful", host)
	return nil
}

func (cl *Cluster) waitForUp(host string) error {
	return wait.PollImmediate(10*time.Second, 1*time.Minute, func() (bool, error) {
		stdout, stderr, err := cl.sshClient.Exec(host, cmdSystemUp)
		if err != nil {
			glog.Errorf("node: %s %v: %v", host, err, stderr)
			return false, nil
		}
		if !bytes.Contains(stdout, []byte("active")) {
			glog.Errorf("node: %s system is not running yet", host)
			return false, nil
		}
		return true, nil
	})
}

func (cl *Cluster) waitForDown(host string) error {
	return wait.PollImmediate(10*time.Second, 1*time.Minute, func() (bool, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, _, err := cl.sshClient.ExecWithCtx(ctx, host, cmdSystemUp)
		if err != nil {
			return true, nil
		}
		return false, nil
	})
}

func (cl *Cluster) rebootHosts(hosts []string, rebootDuration time.Duration) error {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := len(hosts) - 1; i > 0; i-- {
		j := r.Intn(i + 1)
		hosts[i], hosts[j] = hosts[j], hosts[i]
	}

	var errs []error
	errCh := make(chan error)
	errDone := make(chan struct{})
	go func() {
		for err := range errCh {
			errs = append(errs, err)
		}
		errDone <- struct{}{}
	}()

	maxParallel, err := intstr.GetValueFromIntOrPercent(&cl.MaxDisruption, len(hosts), true)
	if err != nil {
		return fmt.Errorf("errors parsing max disruption: %v", err)
	}
	parallel := make(chan struct{}, maxParallel)
	glog.V(4).Infof("parallel reboots: %d", maxParallel)
	var wg sync.WaitGroup
	for i := range hosts {
		wg.Add(1)
		parallel <- struct{}{}
		go func(host string) {
			defer wg.Done()
			if err := cl.RebootNode(host, rebootDuration); err != nil {
				errCh <- err
			}
			<-parallel
		}(hosts[i])
	}
	wg.Wait()
	close(errCh)
	close(parallel)
	<-errDone

	return errors.NewAggregate(errs)
}

func hostsFromNodes(nodes []*v1.Node) (hosts []string) {
	for i := range nodes {
		host := utils.ExternalIP(nodes[i])
		if host == "" {
			glog.Errorf("node: %s has no external IP, will not be rebooted", nodes[i].GetName())
			continue
		}
		hosts = append(hosts, host)
	}

	return hosts
}

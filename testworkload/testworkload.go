package testworkload

import (
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
	batchv1 "k8s.io/client-go/pkg/apis/batch/v1"
	extensionsv1beta1 "k8s.io/client-go/pkg/apis/extensions/v1beta1"
)

const (
	pollTimeout  = 2 * time.Minute
	pollInterval = 5 * time.Second
)

// TestWorkload creates a temp nginx deployment/service pair
// that can be used as a test workload
type TestWorkload struct {
	Namespace string
	Name      string
	// List of pods that belong to the deployment
	Pods []*v1.Pod

	client   kubernetes.Interface
	labelSel *metav1.LabelSelector
}

// New create this nginx deployment/service pair.
// It waits until all the pods in the deployment are running.
func New(kc kubernetes.Interface, namespace string) (*TestWorkload, error) {
	//create random suffix
	name := fmt.Sprintf("nginx-%s", utilrand.String(5))

	tw := &TestWorkload{
		Namespace: namespace,
		Name:      name,
		labelSel: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app": name,
			},
		},
		client: kc,
	}

	//create nginx deployment
	if err := tw.newNginxDeployment(); err != nil && !apierrors.IsAlreadyExists(err) {
		return nil, fmt.Errorf("error creating deployment %s: %v", tw.Name, err)
	}
	if err := wait.PollImmediate(pollInterval, pollTimeout, func() (bool, error) {
		d, err := kc.ExtensionsV1beta1().Deployments(tw.Namespace).Get(tw.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		if d.Status.UpdatedReplicas != d.Status.AvailableReplicas && d.Status.UnavailableReplicas != 0 {
			return false, nil
		}

		return true, nil
	}); err != nil {
		return nil, fmt.Errorf("deployment %s is not ready: %v", tw.Name, err)
	}

	//wait for all pods to enter running phase
	if err := wait.PollImmediate(pollInterval, pollTimeout, func() (bool, error) {
		pl, err := kc.CoreV1().Pods(tw.Namespace).List(metav1.ListOptions{
			LabelSelector: metav1.FormatLabelSelector(tw.labelSel),
		})
		if err != nil {
			return false, err
		}

		if len(pl.Items) == 0 {
			return false, nil
		}

		var pods []*v1.Pod
		for i := range pl.Items {
			p := &pl.Items[i]
			if p.Status.Phase != v1.PodRunning {
				return false, nil
			}

			pods = append(pods, p)
		}

		tw.Pods = pods
		return true, nil
	}); err != nil {
		return nil, fmt.Errorf("pods in deployment %s not ready: %v", tw.Name, err)
	}

	//create nginx service
	if err := tw.newNginxService(); err != nil && !apierrors.IsAlreadyExists(err) {
		return nil, fmt.Errorf("error creating service %s: %v", tw.Name, err)
	}

	return tw, nil
}

// IsReachable pings the nginx service.
// Expects the nginx service to be reachable.
func (tw *TestWorkload) IsReachable() error {
	if err := tw.newPingPod(true); err != nil {
		return fmt.Errorf("error svc wasn't reachable: %v", err)
	}

	return nil
}

// IsUnReachable pings the nginx service.
// Expects the nginx service to be unreachable.
func (tw *TestWorkload) IsUnReachable() error {
	if err := tw.newPingPod(false); err != nil {
		return fmt.Errorf("error svc was reachable: %v", err)
	}

	return nil
}

// Delete deletes the deployment and service
func (tw *TestWorkload) Delete() error {
	delPropPolicy := metav1.DeletePropagationForeground
	if err := wait.PollImmediate(pollInterval, pollTimeout, func() (bool, error) {
		if err := tw.client.ExtensionsV1beta1().Deployments(tw.Namespace).Delete(tw.Name, &metav1.DeleteOptions{
			PropagationPolicy: &delPropPolicy,
		}); err != nil && !apierrors.IsNotFound(err) {
			return false, nil
		}

		if err := tw.client.CoreV1().Services(tw.Namespace).Delete(tw.Name, &metav1.DeleteOptions{
			PropagationPolicy: &delPropPolicy,
		}); err != nil && !apierrors.IsNotFound(err) {
			return false, nil
		}
		return true, nil
	}); err != nil {
		return fmt.Errorf("error deleting %s deployment and serivce: %v", tw.Name, err)
	}

	return nil
}

func (tw *TestWorkload) newNginxDeployment() error {
	var (
		repl  int32 = 2
		cPort int32 = 80
	)
	d := &extensionsv1beta1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tw.Name,
			Namespace: tw.Namespace,
		},
		Spec: extensionsv1beta1.DeploymentSpec{
			Replicas: &repl,
			Selector: tw.labelSel,
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: tw.labelSel.MatchLabels,
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "nginx",
							Image: "nginx:1.12-alpine",
							Ports: []v1.ContainerPort{
								{
									ContainerPort: cPort,
								},
							},
						},
					},
				},
			},
		},
	}
	if _, err := tw.client.ExtensionsV1beta1().Deployments(tw.Namespace).Create(d); err != nil {
		return err
	}

	return nil
}

func (tw *TestWorkload) newNginxService() error {
	var (
		cPort int32 = 80
		tPort int32 = 80
	)
	svc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tw.Name,
			Namespace: tw.Namespace,
		},
		Spec: v1.ServiceSpec{
			Selector: tw.labelSel.MatchLabels,
			Ports: []v1.ServicePort{
				{
					Protocol:   v1.ProtocolTCP,
					Port:       cPort,
					TargetPort: intstr.FromInt(int(tPort)),
				},
			},
		},
	}
	if _, err := tw.client.CoreV1().Services(tw.Namespace).Create(svc); err != nil {
		return err
	}

	return nil
}

func (tw *TestWorkload) newPingPod(reachable bool) error {
	name := fmt.Sprintf("%s-ping-job-%s", tw.Name, utilrand.String(5))
	deadline := int64(pollTimeout.Seconds())

	cmd := fmt.Sprintf("wget --timeout 5 %s", tw.Name)
	if !reachable {
		cmd = fmt.Sprintf("! %s", cmd)
	}
	runcmd := []string{"/bin/sh", "-c"}
	runcmd = append(runcmd, cmd)

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: tw.Namespace,
		},
		Spec: batchv1.JobSpec{
			ActiveDeadlineSeconds: &deadline,
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:    "ping-container",
							Image:   "alpine:3.6",
							Command: runcmd,
						},
					},
					RestartPolicy: v1.RestartPolicyOnFailure,
				},
			},
		},
	}

	if _, err := tw.client.BatchV1().Jobs(tw.Namespace).Create(job); err != nil {
		return err
	}

	// wait for pod state
	if err := wait.PollImmediate(pollInterval, pollTimeout, func() (bool, error) {
		j, err := tw.client.BatchV1().Jobs(tw.Namespace).Get(job.GetName(), metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		if j.Status.Succeeded < 1 {
			return false, nil
		}

		return true, nil
	}); err != nil {
		return fmt.Errorf("ping job didn't succeed: %v", err)
	}

	return nil
}

package nginx

import (
	"fmt"
	"time"

	"github.com/coreos/ktestutil/utils"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilrand "k8s.io/apimachinery/pkg/util/rand"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
	extensionsv1beta1 "k8s.io/client-go/pkg/apis/extensions/v1beta1"
)

// Nginx contains information for nginx deployment/service pair.
type Nginx struct {
	Namespace string
	// Name of the deployment
	Name string
	// Name of the service
	SVCName string
	// List of pods that belong to the deployment
	Pods []*v1.Pod

	kc       kubernetes.Interface
	labelSel *metav1.LabelSelector
}

// NewNginx create this nginx deployment/service pair.
// It waits until all the pods are running.
func NewNginx(kc kubernetes.Interface, namespace string) (*Nginx, error) {
	//create random suffix
	name := fmt.Sprintf("nginx-%s", utilrand.String(5))

	n := &Nginx{
		Namespace: namespace,
		Name:      name,
		SVCName:   name,
		labelSel: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app": name,
			},
		},
		kc: kc,
	}

	//create nginx deployment
	nd := n.getDeploymentSpecs()
	if _, err := kc.ExtensionsV1beta1().Deployments(n.Namespace).Create(nd); err != nil {
		return nil, fmt.Errorf("error creating %s: %v", n.Name, err)
	}
	if err := utils.Retry(10, 5*time.Second, func() error {
		d, err := kc.ExtensionsV1beta1().Deployments(n.Namespace).Get(n.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if d.Status.UpdatedReplicas != d.Status.AvailableReplicas && d.Status.UnavailableReplicas != 0 {
			return fmt.Errorf("expected: %d ready: %d", d.Status.UpdatedReplicas, d.Status.AvailableReplicas)
		}

		return nil
	}); err != nil {
		return nil, fmt.Errorf("error creating %s: %v", n.Name, err)
	}

	//wait for all pods to enter running phase
	if err := utils.Retry(10, 5*time.Second, func() error {
		pl, err := kc.CoreV1().Pods(n.Namespace).List(metav1.ListOptions{
			LabelSelector: metav1.FormatLabelSelector(n.labelSel),
		})
		if err != nil {
			return err
		}

		if len(pl.Items) == 0 {
			return fmt.Errorf("there should be non-zero no. of pods")
		}

		var pods []*v1.Pod
		for i := range pl.Items {
			p := &pl.Items[i]
			if p.Status.Phase != v1.PodRunning {
				return fmt.Errorf("pod %s not running", p.GetName())
			}

			pods = append(pods, p)
		}

		n.Pods = pods
		return nil
	}); err != nil {
		return nil, fmt.Errorf("error creating %s: %v", n.Name, err)
	}

	//create nginx service
	svc := n.getServiceSpecs()
	if _, err := kc.CoreV1().Services(n.Namespace).Create(svc); err != nil {
		return nil, fmt.Errorf("error creating service %s: %v", n.SVCName, err)
	}
	return n, nil
}

// Ping pings the nginx service.
// runs `wget --timeout 5 <nginx-svc-name>`
// expectedPhase is the phase you expect the ping pods to end up in.
func (n *Nginx) Ping(expectedPhase v1.PodPhase) error {
	pp := n.getPingPodSpecs()
	if _, err := n.kc.CoreV1().Pods(n.Namespace).Create(pp); err != nil {
		return err
	}

	// wait for pod state
	if err := utils.Retry(5, 5*time.Second, func() error {
		p, err := n.kc.CoreV1().Pods(n.Namespace).Get(pp.GetName(), metav1.GetOptions{})
		if err != nil {
			return err
		}

		if p.Status.Phase != expectedPhase {
			return fmt.Errorf("expected phase: %v found: %v", expectedPhase, p.Status.Phase)
		}

		return nil
	}); err != nil {
		return err
	}

	return nil
}

// Delete deletes the deployment and service
func (n *Nginx) Delete() error {
	delPropPolicy := metav1.DeletePropagationForeground
	if err := utils.Retry(10, 5*time.Second, func() error {
		if err := n.kc.ExtensionsV1beta1().Deployments(n.Namespace).Delete(n.Name, &metav1.DeleteOptions{
			PropagationPolicy: &delPropPolicy,
		}); err != nil && !apierrors.IsNotFound(err) {
			return err
		}

		if err := n.kc.CoreV1().Services(n.Namespace).Delete(n.SVCName, &metav1.DeleteOptions{
			PropagationPolicy: &delPropPolicy,
		}); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		return nil
	}); err != nil {
		return err
	}

	return nil
}

func (n *Nginx) getDeploymentSpecs() *extensionsv1beta1.Deployment {
	var repl int32 = 2
	var cPort int32 = 80

	return &extensionsv1beta1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      n.SVCName,
			Namespace: n.Namespace,
		},
		Spec: extensionsv1beta1.DeploymentSpec{
			Replicas: &repl,
			Selector: n.labelSel,
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: n.labelSel.MatchLabels,
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
}

func (n *Nginx) getServiceSpecs() *v1.Service {
	var cPort int32 = 80
	var tPort int32 = 80

	return &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      n.Name,
			Namespace: n.Namespace,
		},
		Spec: v1.ServiceSpec{
			Selector: n.labelSel.MatchLabels,
			Ports: []v1.ServicePort{
				{
					Protocol:   v1.ProtocolTCP,
					Port:       cPort,
					TargetPort: intstr.FromInt(int(tPort)),
				},
			},
		},
	}
}

func (n *Nginx) getPingPodSpecs() *v1.Pod {
	name := fmt.Sprintf("%s-ping-pod-%s", n.Name, utilrand.String(5))
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: n.Namespace,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:    "ping-container",
					Image:   "alpine:3.6",
					Command: []string{"wget", "--timeout", "5", n.SVCName},
				},
			},
			RestartPolicy: v1.RestartPolicyNever,
		},
	}
}

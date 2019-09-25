package argocd

import (
	"context"

	argoproj "github.com/jmckind/argocd-operator/pkg/apis/argoproj/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// newDeployment retuns a new Deployment instance.
func newDeployment(name string, namespace string, component string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/component": component,
				"app.kubernetes.io/name":      name,
				"app.kubernetes.io/part-of":   "argocd",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name": name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/name": name,
					},
				},
			},
		},
	}
}

func (r *ReconcileArgoCD) reconcileApplicationControllerDeployment(cr *argoproj.ArgoCD) error {
	deploy := newDeployment("argocd-application-controller", cr.Namespace, "application-controller")
	found := r.isObjectFound(types.NamespacedName{Namespace: cr.Namespace, Name: deploy.Name}, deploy)
	if found {
		// Service found, do nothing
		return nil
	}

	deploy.Spec.Template.Spec.Containers = []corev1.Container{{
		Command: []string{
			"argocd-application-controller",
			"--status-processors",
			"20",
			"--operation-processors",
			"10",
		},
		Image:           "argoproj/argocd:latest",
		ImagePullPolicy: corev1.PullAlways,
		Name:            deploy.Name,
		LivenessProbe: &corev1.Probe{
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/healthz",
					Port: intstr.FromInt(8082),
				},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       10,
		},
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: 8082,
			},
		},
		ReadinessProbe: &corev1.Probe{
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/healthz",
					Port: intstr.FromInt(8082),
				},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       10,
		},
	}}

	deploy.Spec.Template.Spec.ServiceAccountName = deploy.Name

	if err := controllerutil.SetControllerReference(cr, deploy, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), deploy)
}

func (r *ReconcileArgoCD) reconcileDeployments(cr *argoproj.ArgoCD) error {
	err := r.reconcileApplicationControllerDeployment(cr)
	if err != nil {
		return err
	}

	err = r.reconcileDexDeployment(cr)
	if err != nil {
		return err
	}

	err = r.reconcileRedisDeployment(cr)
	if err != nil {
		return err
	}

	err = r.reconcileRepoDeployment(cr)
	if err != nil {
		return err
	}

	err = r.reconcileServerDeployment(cr)
	if err != nil {
		return err
	}

	return nil
}

func (r *ReconcileArgoCD) reconcileDexDeployment(cr *argoproj.ArgoCD) error {
	deploy := newDeployment("argocd-dex-server", cr.Namespace, "dex-server")
	found := r.isObjectFound(types.NamespacedName{Namespace: cr.Namespace, Name: deploy.Name}, deploy)
	if found {
		// Service found, do nothing
		return nil
	}

	deploy.Spec.Template.Spec.Containers = []corev1.Container{{
		Command: []string{
			"/shared/argocd-util",
			"rundex",
		},
		Image:           "quay.io/dexidp/dex:v2.14.0",
		ImagePullPolicy: corev1.PullAlways,
		Name:            "dex",
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: 5556,
			}, {
				ContainerPort: 5557,
			},
		},
		VolumeMounts: []corev1.VolumeMount{{
			Name:      "static-files",
			MountPath: "/shared",
		}},
	}}

	deploy.Spec.Template.Spec.InitContainers = []corev1.Container{{
		Command: []string{
			"cp",
			"/usr/local/bin/argocd-util",
			"/shared",
		},
		Image:           "argoproj/argocd:latest",
		ImagePullPolicy: corev1.PullAlways,
		Name:            "copyutil",
		VolumeMounts: []corev1.VolumeMount{{
			Name:      "static-files",
			MountPath: "/shared",
		}},
	}}

	deploy.Spec.Template.Spec.ServiceAccountName = deploy.Name
	deploy.Spec.Template.Spec.Volumes = []corev1.Volume{{
		Name: "static-files",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}}

	if err := controllerutil.SetControllerReference(cr, deploy, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), deploy)
}

func (r *ReconcileArgoCD) reconcileRedisDeployment(cr *argoproj.ArgoCD) error {
	deploy := newDeployment("argocd-redis", cr.Namespace, "redis")
	found := r.isObjectFound(types.NamespacedName{Namespace: cr.Namespace, Name: deploy.Name}, deploy)
	if found {
		// Service found, do nothing
		return nil
	}

	deploy.Spec.Template.Spec.Containers = []corev1.Container{{
		Args: []string{
			"--save",
			"",
			"--appendonly",
			"no",
		},
		Image:           "redis:5.0.3",
		ImagePullPolicy: corev1.PullAlways,
		Name:            "redis",
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: 6379,
			},
		},
	}}

	if err := controllerutil.SetControllerReference(cr, deploy, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), deploy)
}

func (r *ReconcileArgoCD) reconcileRepoDeployment(cr *argoproj.ArgoCD) error {
	deploy := newDeployment("argocd-repo-server", cr.Namespace, "repo-server")
	found := r.isObjectFound(types.NamespacedName{Namespace: cr.Namespace, Name: deploy.Name}, deploy)
	if found {
		// Service found, do nothing
		return nil
	}

	automountToken := false
	deploy.Spec.Template.Spec.AutomountServiceAccountToken = &automountToken

	deploy.Spec.Template.Spec.Containers = []corev1.Container{{
		Command: []string{
			"argocd-repo-server",
			"--redis",
			"argocd-redis:6379",
		},
		Image:           "argoproj/argocd:latest",
		ImagePullPolicy: corev1.PullAlways,
		LivenessProbe: &corev1.Probe{
			Handler: corev1.Handler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromInt(8081),
				},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       10,
		},
		Name: deploy.Name,
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: 8081,
			}, {
				ContainerPort: 8084,
			},
		},
		ReadinessProbe: &corev1.Probe{
			Handler: corev1.Handler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromInt(8081),
				},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       10,
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "ssh-known-hosts",
				MountPath: "/app/config/ssh",
			}, {
				Name:      "tls-certs",
				MountPath: "/app/config/tls",
			},
		},
	}}

	deploy.Spec.Template.Spec.Volumes = []corev1.Volume{
		{
			Name: "ssh-known-hosts",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "argocd-ssh-known-hosts-cm",
					},
				},
			},
		}, {
			Name: "tls-certs",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "argocd-tls-certs-cm",
					},
				},
			},
		},
	}

	if err := controllerutil.SetControllerReference(cr, deploy, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), deploy)
}

func (r *ReconcileArgoCD) reconcileServerDeployment(cr *argoproj.ArgoCD) error {
	deploy := newDeployment("argocd-server", cr.Namespace, "server")
	found := r.isObjectFound(types.NamespacedName{Namespace: cr.Namespace, Name: deploy.Name}, deploy)
	if found {
		// Service found, do nothing
		return nil
	}

	deploy.Spec.Template.Spec.Containers = []corev1.Container{{
		Command: []string{
			"argocd-server",
			"--staticassets",
			"/shared/app",
		},
		Image:           "argoproj/argocd:latest",
		ImagePullPolicy: corev1.PullAlways,
		LivenessProbe: &corev1.Probe{
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/healthz",
					Port: intstr.FromInt(8080),
				},
			},
			InitialDelaySeconds: 3,
			PeriodSeconds:       30,
		},
		Name: deploy.Name,
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: 8080,
			}, {
				ContainerPort: 8083,
			},
		},
		ReadinessProbe: &corev1.Probe{
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/healthz",
					Port: intstr.FromInt(8080),
				},
			},
			InitialDelaySeconds: 3,
			PeriodSeconds:       30,
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "ssh-known-hosts",
				MountPath: "/app/config/ssh",
			}, {
				Name:      "tls-certs",
				MountPath: "/app/config/tls",
			},
		},
	}}

	deploy.Spec.Template.Spec.ServiceAccountName = deploy.Name
	deploy.Spec.Template.Spec.Volumes = []corev1.Volume{
		{
			Name: "ssh-known-hosts",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "argocd-ssh-known-hosts-cm",
					},
				},
			},
		}, {
			Name: "tls-certs",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "argocd-tls-certs-cm",
					},
				},
			},
		},
	}

	if err := controllerutil.SetControllerReference(cr, deploy, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), deploy)
}

package kube

import (
	"k8s.io/client-go/kubernetes"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

// Clienset returns a kubernetes clientset
func Clienset() kubernetes.Interface {
	restconfig := controllerruntime.GetConfigOrDie()

	return kubernetes.NewForConfigOrDie(restconfig)
}

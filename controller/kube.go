package controller

import (
	"github.com/wutong-paas/wutong-controller/option"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func KubeClient(conf *option.Config) *kubernetes.Clientset {
	config, err := clientcmd.BuildConfigFromFlags("", conf.KubeConfig)
	if err != nil {
		config, err = rest.InClusterConfig()
		if err != nil {
			panic(err.Error())
		}
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	return clientset
}

package main

import (
	"flag"
	"time"

	_ "net/http/pprof"

	"github.com/wutong-paas/wutong-controller/controller"
	"github.com/wutong-paas/wutong-controller/option"
)

func main() {
	conf := BuildConfigFromFlags()

	scc := controller.NewServiceCombinerController(conf)
	scc.Run()
}

func BuildConfigFromFlags() *option.Config {
	// kubeconfig := flag.String("kubeconfig", "/root/.kube/config", "(optional) location to your kubeconfig file")
	kubeconfig := flag.String("kubeconfig", "/root/source/wutong-paas/wutong-controller/kubeconfig", "kubeconfig file path")
	resyncPeroid := flag.Duration("resync-period", 10*time.Minute, "resync period")
	flag.Parse()

	return &option.Config{
		KubeConfig:   *kubeconfig,
		ResyncPeriod: *resyncPeroid,
	}
}

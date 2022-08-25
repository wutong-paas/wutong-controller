package main

import (
	"flag"
	"fmt"
	"sync"
	"time"

	_ "net/http/pprof"

	"github.com/wutong-paas/wutong-controller/controller"
	"github.com/wutong-paas/wutong-controller/option"
	"github.com/wutong-paas/wutong-controller/pkg/kube"
)

func main() {
	conf := BuildConfigFromFlags()
	scc := controller.NewServiceCombinerController(conf)
	tprbacc := controller.NewTelepresenceRBACController(conf)
	controllers := []controller.ControllerInterface{
		scc,
		tprbacc,
	}

	var wg sync.WaitGroup
	for _, c := range controllers {
		wg.Add(1)
		go func(c controller.ControllerInterface, wg *sync.WaitGroup) {
			defer wg.Done()
			fmt.Println(111)
			c.Run()
			fmt.Println(222)
		}(c, &wg)
	}
	wg.Wait()
}

func BuildConfigFromFlags() *option.Config {
	resyncPeroid := flag.Duration("resync-period", 10*time.Minute, "resync period")
	flag.Parse()

	fmt.Printf("resyncPeroid: %v\n", resyncPeroid)
	return &option.Config{
		KubeClient:   kube.Clienset(),
		ResyncPeriod: *resyncPeroid,
	}
}

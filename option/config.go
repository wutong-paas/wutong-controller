package option

import (
	"time"

	"k8s.io/client-go/kubernetes"
)

type Config struct {
	KubeClient   kubernetes.Interface
	ResyncPeriod time.Duration
}

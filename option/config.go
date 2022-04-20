package option

import "time"

type Config struct {
	KubeConfig   string
	ResyncPeriod time.Duration
}

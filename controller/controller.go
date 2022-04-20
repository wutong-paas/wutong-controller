package controller

import (
	"context"
	"strings"
	"time"

	"github.com/wutong-paas/wutong-controller/option"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

// Combine the inner services created by Wutong, so the component
// can get the special service name for inner access.
type ServiceCombinerController struct {
	clientset          kubernetes.Interface
	serviceCacheSynced cache.InformerSynced
	serviceLister      corelisters.ServiceLister
	queue              workqueue.RateLimitingInterface
	stopC              chan struct{}
}

func NewServiceCombinerController(conf *option.Config) *ServiceCombinerController {
	clientset := KubeClient(conf)
	informerFactory := informers.NewSharedInformerFactoryWithOptions(clientset, conf.ResyncPeriod, informers.WithTweakListOptions(func(lo *metav1.ListOptions) {
		lo.LabelSelector = "creator=Wutong,service_type=inner"
	}))
	stopC := make(chan struct{})

	serviceInformer := informerFactory.Core().V1().Services()
	c := &ServiceCombinerController{
		clientset:          clientset,
		serviceCacheSynced: serviceInformer.Informer().HasSynced,
		serviceLister:      serviceInformer.Lister(),
		queue:              workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "service-combiner"),
		stopC:              stopC,
	}
	serviceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.queue.Add(obj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			c.queue.Add(newObj)
		},
		DeleteFunc: func(obj interface{}) {
			c.queue.Add(obj)
		},
	})

	informerFactory.Start(stopC)
	return c
}

func (c *ServiceCombinerController) Run() {
	if !cache.WaitForCacheSync(c.stopC, c.serviceCacheSynced) {
		klog.Infoln("waiting cache to be synced.")
	}
	go wait.Until(c.worker, time.Second, c.stopC)
	<-c.stopC
}

func (c *ServiceCombinerController) worker() {
	for c.processItem() {

	}
}

func (c *ServiceCombinerController) processItem() bool {
	item, shutdown := c.queue.Get()
	if shutdown {
		return false
	}

	defer c.queue.Forget(item)

	if ok := c.syncService(item); !ok {
		klog.Infoln("syncing service failed.")
		return false
	}
	return true
}

func (c *ServiceCombinerController) syncService(obj interface{}) bool {
	svc, ok := obj.(*corev1.Service)
	if !ok {
		return false
	}
	cs, ok := c.getCombinedService(svc)
	if !ok {
		return false
	}
	if cs != nil {
		selector := labels.SelectorFromSet(labels.Set{
			"creator":       "Wutong",
			"service_type":  "inner",
			"app":           appName(svc),
			"service_alias": serviceName(svc),
		})
		selector.Add()
		svcGroups, err := c.serviceLister.Services(svc.Namespace).List(selector)
		if err != nil {
			klog.Infof("list service err: %s", err.Error())
			return false
		}

		if len(svcGroups) > 0 {
			portMap := make(map[int32]corev1.ServicePort)
			for _, svc := range svcGroups {
				for _, p := range svc.Spec.Ports {
					portMap[p.Port] = p
				}
			}
			newPorts := make([]corev1.ServicePort, 0)
			for _, v := range portMap {
				newPorts = append(newPorts, v)
			}

			cs.Spec.Ports = newPorts
			return c.updateCombinedService(cs)
		} else {
			// delete combined service
			return c.deleteCombinedService(cs)
		}
	} else {
		selector := labels.SelectorFromSet(labels.Set{
			"creator":       "Wutong",
			"service_type":  "inner",
			"app":           appName(svc),
			"service_alias": serviceName(svc),
		})
		selector.Add()
		svcGroups, err := c.serviceLister.Services(svc.Namespace).List(selector)
		if err != nil {
			klog.Infof("list service err: %s", err.Error())
			return false
		}
		if len(svcGroups) > 0 {
			portMap := make(map[int32]corev1.ServicePort)
			for _, svc := range svcGroups {
				for _, p := range svc.Spec.Ports {
					portMap[p.Port] = p
				}
			}
			newPorts := make([]corev1.ServicePort, 0)
			for _, v := range portMap {
				newPorts = append(newPorts, v)
			}

			cs = &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      combinedServiceName(svc),
					Namespace: svc.Namespace,
					Labels: map[string]string{
						"creator":       "wutong-controller",
						"app":           appName(svc),
						"service_alias": serviceName(svc),
					},
				},
				Spec: corev1.ServiceSpec{
					Ports:    newPorts,
					Selector: svc.Spec.Selector,
					Type:     corev1.ServiceTypeClusterIP,
				},
			}
			return c.addCombinedService(cs)
		}
		return true
	}
}

func (c *ServiceCombinerController) addCombinedService(cs *corev1.Service) bool {
	if _, err := c.clientset.CoreV1().Services(cs.Namespace).Create(context.Background(), cs, metav1.CreateOptions{}); err != nil {
		klog.Infof("creating combined service err: %s.", err.Error())
		return false
	}
	klog.Infof("created combined service [%s/%s].", cs.Namespace, cs.Name)
	return true
}

func (c *ServiceCombinerController) deleteCombinedService(cs *corev1.Service) bool {
	if err := c.clientset.CoreV1().Services(cs.Namespace).Delete(context.Background(), cs.Name, metav1.DeleteOptions{}); err != nil {
		klog.Infof("delete combined service [%s/%s] err: %s.", cs.Namespace, cs.Name, err.Error())
		return false
	}
	klog.Infof("deleted combined service [%s/%s].", cs.Namespace, cs.Name)
	return true
}

func (c *ServiceCombinerController) updateCombinedService(cs *corev1.Service) bool {
	if _, err := c.clientset.CoreV1().Services(cs.Namespace).Update(context.Background(), cs, metav1.UpdateOptions{}); err != nil {
		klog.Infof("updated combined service err: %s", err.Error())
		return false
	}
	klog.Infof("updated combined service [%s/%s].", cs.Namespace, cs.Name)
	return true
}

func appName(svc *corev1.Service) string {
	return svc.Labels["app"]
}

func serviceName(svc *corev1.Service) string {
	return svc.Labels["service_alias"]
}

func combinedServiceName(svc *corev1.Service) string {
	return strings.Join([]string{"wtsvc", serviceName(svc)}, "-")
}

func (c *ServiceCombinerController) getCombinedService(svc *corev1.Service) (*corev1.Service, bool) {
	combinedService, err := c.clientset.CoreV1().Services(svc.Namespace).Get(context.Background(), combinedServiceName(svc), metav1.GetOptions{})
	if err != nil {
		return nil, apierrors.IsNotFound(err)
	}
	return combinedService, true
}

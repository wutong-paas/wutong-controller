package controller

import (
	"context"
	"time"

	"github.com/wutong-paas/wutong-controller/option"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

// TelepresenceRBACController control telepresence rbac service account for team.
type TelepresenceRBACController struct {
	clientset            kubernetes.Interface
	namespaceCacheSynced cache.InformerSynced
	namespaceLister      corelisters.NamespaceLister
	queue                workqueue.RateLimitingInterface
	stopC                chan struct{}
}

func NewTelepresenceRBACController(conf *option.Config) *TelepresenceRBACController {
	informerFactory := informers.NewSharedInformerFactoryWithOptions(conf.KubeClient, conf.ResyncPeriod, informers.WithTweakListOptions(func(lo *metav1.ListOptions) {
		lo.LabelSelector = "app.kubernetes.io/managed-by=wutong"
	}))
	stopC := make(chan struct{})

	namespaceInformer := informerFactory.Core().V1().Namespaces()
	c := &TelepresenceRBACController{
		clientset:            conf.KubeClient,
		namespaceCacheSynced: namespaceInformer.Informer().HasSynced,
		namespaceLister:      namespaceInformer.Lister(),
		queue:                workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "telepresence-rbac-controller"),
		stopC:                stopC,
	}

	namespaceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
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

func (c *TelepresenceRBACController) Run() {
	klog.Infoln("Starting telepresence rbac controller")
	if !cache.WaitForCacheSync(c.stopC, c.namespaceCacheSynced) {
		klog.Infoln("waiting cache to be synced.")
	}
	go wait.Until(c.worker, time.Second, c.stopC)
	<-c.stopC
}

func (c *TelepresenceRBACController) worker() {
	for c.processItem() {

	}
}

func (c *TelepresenceRBACController) processItem() bool {
	item, shutdown := c.queue.Get()
	if shutdown {
		return false
	}

	defer c.queue.Forget(item)

	if ok := c.syncRBAC(item); !ok {
		klog.Infoln("syncing rbac failed.")
		return false
	}
	return true
}

func (c *TelepresenceRBACController) syncRBAC(obj interface{}) bool {
	ns, ok := obj.(*corev1.Namespace)
	if !ok {
		return false
	}
	rbac := newRBAC(ns)

	_, err := c.clientset.CoreV1().Namespaces().Get(context.TODO(), ns.Name, metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return false
		}
		// namespace could be deleted, delete rbac
		c.clientset.CoreV1().ServiceAccounts(tpNamespace).Delete(context.TODO(), rbac.tpsa.Name, metav1.DeleteOptions{})
		c.clientset.RbacV1().RoleBindings(tpNamespace).Delete(context.TODO(), rbac.tprb.Name, metav1.DeleteOptions{})
		c.clientset.RbacV1().ClusterRoleBindings().Delete(context.TODO(), rbac.crb.Name, metav1.DeleteOptions{})
	} else {
		// apply telepresence dev rbac for every namespace.
		if _, err := c.clientset.CoreV1().ServiceAccounts(tpNamespace).Get(context.TODO(), rbac.tpsa.Name, metav1.GetOptions{}); err != nil {
			if errors.IsNotFound(err) {
				_, err := c.clientset.CoreV1().ServiceAccounts(tpNamespace).Create(context.TODO(), rbac.tpsa, metav1.CreateOptions{})
				if err != nil {
					klog.Errorf("create service account %s failed: %v", saName(ns.Name), err)
					return false
				}
				klog.Infof("create service account %s success", saName(ns.Name))
			} else {
				klog.Errorf("get service account %s failed: %v", saName(ns.Name), err)
				return false
			}
		}

		if _, err := c.clientset.RbacV1().ClusterRoles().Get(context.TODO(), tpdev4rb.Name, metav1.GetOptions{}); err != nil {
			if errors.IsNotFound(err) {
				_, err := c.clientset.RbacV1().ClusterRoles().Create(context.TODO(), tpdev4rb, metav1.CreateOptions{})
				if err != nil {
					klog.Errorf("create cluster role %s failed: %v", tpdev4rb.Name, err)
					return false
				}
				klog.Infof("create cluster role %s success", tpdev4rb.Name)
			} else {
				klog.Errorf("get cluster role %s failed: %v", tpdev4rb.Name, err)
				return false
			}
		}

		if _, err := c.clientset.RbacV1().RoleBindings(rbac.tprb.Namespace).Get(context.TODO(), rbac.tprb.Name, metav1.GetOptions{}); err != nil {
			if errors.IsNotFound(err) {
				_, err := c.clientset.RbacV1().RoleBindings(rbac.tprb.Namespace).Create(context.TODO(), rbac.tprb, metav1.CreateOptions{})
				if err != nil {
					klog.Errorf("create role binding %s failed: %v", rbac.tprb.Name, err)
					return false
				}
				klog.Infof("create role binding %s success", rbac.tprb.Name)
			} else {
				klog.Errorf("get role binding %s failed: %v", rbac.tprb.Name, err)
				return false
			}
		}

		if _, err := c.clientset.RbacV1().RoleBindings(rbac.nsrb.Namespace).Get(context.TODO(), rbac.nsrb.Name, metav1.GetOptions{}); err != nil {
			if errors.IsNotFound(err) {
				_, err := c.clientset.RbacV1().RoleBindings(rbac.nsrb.Namespace).Create(context.TODO(), rbac.nsrb, metav1.CreateOptions{})
				if err != nil {
					klog.Errorf("create role binding %s failed: %v", rbac.nsrb.Name, err)
					return false
				}
				klog.Infof("create role binding %s success", rbac.nsrb.Name)
			} else {
				klog.Errorf("get role binding %s failed: %v", rbac.nsrb.Name, err)
				return false
			}
		}

		if _, err := c.clientset.RbacV1().ClusterRoles().Get(context.TODO(), tpdev4crb.Name, metav1.GetOptions{}); err != nil {
			if errors.IsNotFound(err) {
				_, err := c.clientset.RbacV1().ClusterRoles().Create(context.TODO(), tpdev4crb, metav1.CreateOptions{})
				if err != nil {
					klog.Errorf("create cluster role %s failed: %v", tpdev4crb.Name, err)
					return false
				}
				klog.Infof("create cluster role %s success", tpdev4crb.Name)
			} else {
				klog.Errorf("get cluster role %s failed: %v", tpdev4crb.Name, err)
				return false
			}
		}

		if _, err := c.clientset.RbacV1().ClusterRoleBindings().Get(context.TODO(), rbac.crb.Name, metav1.GetOptions{}); err != nil {
			if errors.IsNotFound(err) {
				_, err := c.clientset.RbacV1().ClusterRoleBindings().Create(context.TODO(), rbac.crb, metav1.CreateOptions{})
				if err != nil {
					klog.Errorf("create cluster role binding %s failed: %v", rbac.crb.Name, err)
					return false
				}
				klog.Infof("create cluster role binding %s success", rbac.crb.Name)
			} else {
				klog.Errorf("get cluster role binding %s failed: %v", rbac.crb.Name, err)
				return false
			}
		}
	}

	return true
}

type telepresenceRBAC struct {
	tpsa *corev1.ServiceAccount
	tprb *rbacv1.RoleBinding
	nsrb *rbacv1.RoleBinding
	crb  *rbacv1.ClusterRoleBinding
}

var (
	tpNamespace = "ambassador"
)

func newRBAC(ns *corev1.Namespace) telepresenceRBAC {
	sa := saName(ns.Name)

	rbac := telepresenceRBAC{
		tpsa: &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "tpdev-" + ns.Name,
				Namespace: tpNamespace,
			},
		},
		tprb: &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "tpdev-tprb-" + ns.Name,
				Namespace: tpNamespace,
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      sa,
					Namespace: tpNamespace,
				},
			},
			RoleRef: rbacv1.RoleRef{
				Kind:     "ClusterRole",
				Name:     tpdev4rb.Name,
				APIGroup: "rbac.authorization.k8s.io",
			},
		},
		nsrb: &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "tpdev-nsrb-" + ns.Name,
				Namespace: ns.Name,
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      sa,
					Namespace: tpNamespace,
				},
			},
			RoleRef: rbacv1.RoleRef{
				Kind:     "ClusterRole",
				Name:     tpdev4rb.Name,
				APIGroup: "rbac.authorization.k8s.io",
			},
		},
		crb: &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "tpdev-crb-" + ns.Name,
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      sa,
					Namespace: tpNamespace,
				},
			},
			RoleRef: rbacv1.RoleRef{
				Kind:     "ClusterRole",
				Name:     tpdev4crb.Name,
				APIGroup: "rbac.authorization.k8s.io",
			},
		},
	}
	return rbac
}

func saName(ns string) string {
	return "tpdev-" + ns
}

var (
	tpdev4rb = &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "tpdev4rb",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"pods"},
				Verbs:     []string{"get", "list", "watch", "create", "delete"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"services"},
				Verbs:     []string{"update"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"pods/portforward"},
				Verbs:     []string{"create"},
			},
			{
				APIGroups: []string{"apps"},
				Resources: []string{"deployments", "statefulsets", "replicasets"},
				Verbs:     []string{"get", "list", "watch", "update"},
			},
			{
				APIGroups: []string{"getambassador.io"},
				Resources: []string{"hosts", "mappings"},
				Verbs:     []string{"*"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"endpoints"},
				Verbs:     []string{"get", "list", "watch"},
			},
		},
	}
	tpdev4crb = &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "tpdev4crb",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"namespaces"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"services"},
				Verbs:     []string{"get", "list", "watch"},
			},
		},
	}
)

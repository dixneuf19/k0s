package controller

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"

	cfgClient "github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/clientset"
	"github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0s/pkg/constant"
	k8sutil "github.com/k0sproject/k0s/pkg/kubernetes"
)

// ClusterConfigReconciler reconciles a ClusterConfig object
type ClusterConfigReconciler struct {
	ClusterConfig *v1beta1.ClusterConfig

	configClient      *cfgClient.K0sV1beta1Client
	kubeClientFactory k8sutil.ClientFactory
	leaderElector     LeaderElector
	log               *logrus.Entry
	restConfig        *rest.Config

	tickerDone chan struct{}
}

// NewClusterConfigReconciler creates a new clusterConfig reconciler
func NewClusterConfigReconciler(c *v1beta1.ClusterConfig, leaderElector LeaderElector, kubeClientFactory k8sutil.ClientFactory) *ClusterConfigReconciler {
	d := atomic.Value{}
	d.Store(true)

	return &ClusterConfigReconciler{
		ClusterConfig: c,

		kubeClientFactory: kubeClientFactory,
		leaderElector:     leaderElector,
		log:               logrus.WithFields(logrus.Fields{"component": "clusterConfig-reconciler"}),
	}
}

func (r *ClusterConfigReconciler) Init() error {
	return nil
}

func (r *ClusterConfigReconciler) Run() error {
	restConfig, err := r.kubeClientFactory.GetRestConfig()
	// failed to get config Client
	if err != nil {
		return fmt.Errorf("failed to set config client: %v", err)
	}

	r.configClient = cfgClient.NewForConfigOqrDie(restConfig)
	r.restConfig = restConfig

	r.tickerDone = make(chan struct{})
	go r.Reconcile()
	return nil
}

//+kubebuilder:rbac:groups=k0s.k0sproject.io,resources=clusterconfigs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=k0s.k0sproject.io,resources=clusterconfigs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=k0s.k0sproject.io,resources=clusterconfigs/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ClusterConfig object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *ClusterConfigReconciler) Reconcile() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			opts := v1.ListOptions{}
			cfgList, err := r.configClient.ClusterConfigs(constant.ClusterConfigNamespace).List(context.Background(), opts)
			if err != nil {
				r.log.Errorf("failed to reconcile config status: %v", err)
				continue
			}
			r.log.Infof("got config list: %+v\n", cfgList)

		case <-r.tickerDone:
			r.log.Info("clusterConfig reconciler done")
			return
		}
	}
}

// Stop stops
func (r *ClusterConfigReconciler) Stop() error {
	if r.tickerDone != nil {
		close(r.tickerDone)
	}
	return nil
}

func (r *ClusterConfigReconciler) Healthy() error {
	return nil
}

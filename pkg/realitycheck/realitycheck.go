package realitycheck

import (
	"fmt"
	"strconv"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	informers_v1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/golang/glog"
	"github.com/sapcc/helm-realitycheck/pkg/helm"
	"github.com/sapcc/helm-realitycheck/pkg/release"
)

type Checker struct {
	helmClient            *helm.Client
	kubeClient            kubernetes.Interface
	flags                 *genericclioptions.ConfigFlags
	helmConfigMapInformer cache.SharedIndexInformer

	trackedReleased map[string]*release.HelmRelease
}

func New(flags *genericclioptions.ConfigFlags) *Checker {
	return &Checker{flags: flags}
}

func (c *Checker) setup() error {

	kubeConfig, err := c.flags.ToRESTConfig()
	if err != nil {
		return err
	}

	c.kubeClient, err = kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return err
	}

	//c.helmClient, err = helm.NewClient(c.kubeClient, kubeConfig)
	//if err != nil {
	//  return err
	//}

	return nil

}

// MetaNamespaceIndexFunc is a default index function that indexes based on an object's namespace
func HelmReleaseIndexFunc(obj interface{}) ([]string, error) {
	meta, err := meta.Accessor(obj)
	if err != nil {
		return []string{""}, fmt.Errorf("object has no meta: %v", err)
	}
	return []string{meta.GetLabels()["NAME"]}, nil
}

func (c *Checker) Run(stopCh <-chan struct{}) error {
	err := c.setup()
	if err != nil {
		return err
	}

	filterOptions := func(m *meta_v1.ListOptions) {
		m.LabelSelector = "OWNER=TILLER, STATUS in (DEPLOYED, FAILED)"
	}
	c.helmConfigMapInformer = informers_v1.NewFilteredConfigMapInformer(c.kubeClient, "kube-system", 5*time.Minute, cache.Indexers{"release": HelmReleaseIndexFunc}, filterOptions)

	c.helmConfigMapInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				glog.Info("ADDED", key)
				//c.queue.Add(key)
			}
		},
		UpdateFunc: func(old interface{}, new interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(new)
			if err == nil {
				glog.Info("UPDATED", key)
				//c.queue.Add(key)
			}
		},
		DeleteFunc: func(obj interface{}) {
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err == nil {
				glog.Info("DELETED", key)
				//c.queue.Add(key)
			}
		},
	})

	glog.Info("Starting configmap informer")
	go c.helmConfigMapInformer.Run(stopCh)
	cache.WaitForCacheSync(stopCh, c.helmConfigMapInformer.HasSynced)

	glog.Info("Configmap informer synced")
	c.trackedReleased = make(map[string]*release.HelmRelease, len(c.getReleases()))
	for _, name := range c.getReleases() {
		c.trackedReleased[name] = release.NewHelmRelease(name, "", "")
	}

	return nil
}

func (c *Checker) getReleases() []string {
	return c.helmConfigMapInformer.GetIndexer().ListIndexFuncValues("release")
}

func (c *Checker) getLatestRelease(release string) (*v1.ConfigMap, error) {
	configmaps, err := c.helmConfigMapInformer.GetIndexer().ByIndex("release", release)
	if err != nil {
		return nil, err
	}
	if len(configmaps) == 0 {
		return nil, fmt.Errorf("No configmap for release %s found", release)
	}

	var latest *v1.ConfigMap
	for _, obj := range configmaps {
		current := obj.(*v1.ConfigMap)
		if latest == nil {
			latest = current
			continue
		}
		latestVersion, err := strconv.Atoi(latest.Labels["VERSION"])
		if err != nil || latestVersion == 0 {
			return nil, fmt.Errorf("Configmap %s does not have required VERSION label", latest.GetName())
		}
		currentVersion, err := strconv.Atoi(current.Labels["VERSION"])
		if err != nil || currentVersion == 0 {
			return nil, fmt.Errorf("Configmaps %s for release %s do not have required VERSION label", current.GetName())
		}

		if currentVersion > latestVersion {
			latest = current
		}
	}
	return latest, nil

}

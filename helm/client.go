package helm

import (
	"fmt"
	"os"

	"github.com/golang/glog"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/helm/pkg/helm"

	"github.com/sapcc/helm-realitycheck/helm/portforwarder"
)

type Client struct {
	*helm.Client
	tunnel *portforwarder.Tunnel
}

func (c *Client) Close() {
	if c.tunnel != nil {
		glog.V(2).Infof("tearing down tunnel to tiller pod %s/%s", c.tunnel.Namespace, c.tunnel.PodName)
		c.tunnel.Close()
	}
}

func NewClient(kubeClient kubernetes.Interface, kubeConfig *rest.Config) (*Client, error) {

	tillerHost := os.Getenv("TILLER_DEPLOY_SERVICE_HOST")
	if tillerHost == "" {
		tillerHost = "tiller-deploy.kube-system"
	}
	tillerPort := os.Getenv("TILLER_DEPLOY_SERVICE_PORT")
	if tillerPort == "" {
		tillerPort = "44134"
	}
	tillerHost = fmt.Sprintf("%s:%s", tillerHost, tillerPort)

	if _, err := rest.InClusterConfig(); err != nil {
		glog.V(2).Info("We are not running inside the cluster. Creating tunnel to tiller pod.")
		tunnel, err := portforwarder.New("kube-system", kubeClient, kubeConfig)
		if err != nil {
			return nil, err
		}
		tillerHost = fmt.Sprintf("localhost:%d", tunnel.Local)
		client := helm.NewClient(helm.Host(tillerHost))
		return &Client{client, tunnel}, nil
	}
	return &Client{helm.NewClient(helm.Host(tillerHost)), nil}, nil
}

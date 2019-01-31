package main

import (
	"flag"
	"fmt"
	"log"
	"strings"

	"github.com/d4l3k/messagediff"
	"github.com/golang/glog"
	"github.com/spf13/pflag"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/genericclioptions/resource"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/sapcc/helm-realitycheck/helm"
)

func main() {

	if f := flag.Lookup("logtostderr"); f != nil {
		f.Value.Set("true")
	}
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)

	getter := genericclioptions.NewConfigFlags()

	getter.AddFlags(pflag.CommandLine)

	pflag.Parse()

	kubeConfig, err := getter.ToRESTConfig()
	if err != nil {
		log.Fatal(err)
	}

	kubeClient, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		glog.Fatal(err)
	}

	helmClient, err := helm.NewClient(kubeClient, kubeConfig)
	if err != nil {
		glog.Fatal(err)
	}
	defer helmClient.Close()

	releaseResponse, err := helmClient.ReleaseContent("kubernikus")
	if err != nil {
		glog.Fatal(err)
	}

	release := releaseResponse.GetRelease()
	if release == nil {
		glog.Fatal("release was nil")
	}

	builder := resource.NewBuilder(getter).
		ContinueOnError().
		WithScheme(scheme.Scheme, scheme.Scheme.PrioritizedVersionsAllGroups()...).
		//Schema(c.validator()).
		NamespaceParam(release.GetNamespace()).
		DefaultNamespace().
		Stream(strings.NewReader(release.GetManifest()), fmt.Sprintf("release %s", release.GetName())).
		Flatten()

	result := builder.Do()
	infos, err := result.Infos()
	if err != nil {
		log.Fatal(err)
	}

	info := infos[0]
	helper := resource.NewHelper(info.Client, info.Mapping)
	fromServer, err := helper.Get(info.Namespace, info.Name, false)
	if err != nil {
		log.Fatal(err)
	}

	diff, _ := messagediff.PrettyDiff(info.Object, fromServer, messagediff.IgnoreStructField("CreationTimestamp"))
	fmt.Println(diff)

	//spew.Dump(fromServer)
}

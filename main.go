package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/go-test/deep"
	"github.com/golang/glog"
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/genericclioptions/resource"
)

func main() {

	if f := flag.Lookup("logtostderr"); f != nil {
		f.Value.Set("true")
	}
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)

	getter := genericclioptions.NewConfigFlags()

	getter.AddFlags(pflag.CommandLine)

	pflag.Parse()

	//kubeConfig, err := getter.ToRESTConfig()
	//if err != nil {
	//  log.Fatal(err)
	//}

	//kubeClient, err := kubernetes.NewForConfig(kubeConfig)
	//if err != nil {
	//  glog.Fatal(err)
	//}

	//helmClient, err := helm.NewClient(kubeClient, kubeConfig)
	//if err != nil {
	//  glog.Fatal(err)
	//}
	//defer helmClient.Close()

	//releaseResponse, err := helmClient.ReleaseContent(pflag.Arg(0))
	//if err != nil {
	//  glog.Fatal(err)
	//}

	//release := releaseResponse.GetRelease()
	//if release == nil {
	//  glog.Fatal("release was nil")
	//}

	//stream := strings.Reader(release.GetManifest())
	//namespace := release.GetNamespace()
	//release := release.GetName()

	file, err := os.Open(pflag.Arg(0))
	if err != nil {
		glog.Fatal(err)
	}
	defer file.Close()

	stream := file
	namespace := "monsoon3"
	release := pflag.Arg(0)

	builder := resource.NewBuilder(getter).
		ContinueOnError().
		Unstructured().
		//WithScheme(scheme.Scheme, scheme.Scheme.PrioritizedVersionsAllGroups()...).
		//Schema(c.validator()).
		NamespaceParam(namespace).
		DefaultNamespace().
		Stream(stream, fmt.Sprintf("release %s", release)).
		Flatten()

	result := builder.Do()

	infos, err := result.Infos()
	if err != nil && infos == nil {
		glog.Fatal(err)
	}

	//apiTypes := make(map[string]bool, 0)
	//for _, info := range infos {
	//  fmt.Println("%#v", info.Object)
	//  gvk := info.Object.GetObjectKind().GroupVersionKind()
	//  g := fmt.Sprintf("%s/%s", gvk.Version, gvk.Kind)
	//  apiTypes[g] = true
	//}

	//for t, _ := range apiTypes {
	//  fmt.Println(t)
	//}

	for _, info := range infos {

		object := info.Object
		gvk := object.GetObjectKind().GroupVersionKind()

		helper := resource.NewHelper(info.Client, info.Mapping)
		fromServer, err := helper.Get(info.Namespace, info.Name, false)
		if err != nil {
			log.Fatal(err)
		}

		//fmt.Printf("%#v", fromServer)

		filteredFromServer := fromServer.(*unstructured.Unstructured)
		deleteKey(filteredFromServer.Object, "metadata.selfLink")
		deleteKey(filteredFromServer.Object, "metadata.creationTimestamp")
		deleteKey(filteredFromServer.Object, "metadata.deletionTimestamp")
		deleteKey(filteredFromServer.Object, "metadata.clusterName")
		deleteKey(filteredFromServer.Object, "metadata.generation")
		deleteKey(filteredFromServer.Object, "metadata.uid")
		deleteKey(filteredFromServer.Object, "metadata.deletionGracePeriodSeconds")
		deleteKey(filteredFromServer.Object, "metadata.resourceVersion")

		diffs := deep.Equal(object, filteredFromServer)

		if len(diffs) > 0 {
			fmt.Println(info.Object.GetObjectKind().GroupVersionKind().String(), " is different")
			for _, diff := range deep.Equal(info.Object, filteredFromServer) {
				fmt.Println(diff)
			}
		} else {
			fmt.Println(gvk.String(), " OK")

		}
		//diff, _ := messagediff.PrettyDiff(info.Object, fromServer)
		//fmt.Println(diff)

	}

	//spew.Dump(fromServer)
}

func get(current interface{}, selector string) interface{} {
	selSegs := strings.SplitN(selector, ".", 2)
	thisSel := selSegs[0]
	switch current.(type) {
	case map[string]interface{}:
		curMSI := current.(map[string]interface{})
		if len(selSegs) <= 1 {
			return curMSI[thisSel]
		}
		current = curMSI[thisSel]

	default:
		current = nil
	}
	if len(selSegs) > 1 {
		current = get(current, selSegs[1])
	}
	return current
}

// access accesses the object using the selector and performs the
// appropriate action.
func deleteKey(current interface{}, selector string) interface{} {
	selSegs := strings.SplitN(selector, ".", 2)
	thisSel := selSegs[0]
	//index := -1

	//if strings.Contains(thisSel, "[") {
	//  index, thisSel = getIndex(thisSel)
	//}

	//if curMap, ok := current.(Map); ok {
	//  current = map[string]interface{}(curMap)
	//}
	// get the object in question
	switch current.(type) {
	case map[string]interface{}:
		curMSI := current.(map[string]interface{})
		if len(selSegs) <= 1 {
			delete(curMSI, thisSel)
			return nil
		}

		//_, ok := curMSI[thisSel].(map[string]interface{})
		//if (curMSI[thisSel] == nil || !ok) && index == -1 && isSet {
		//  curMSI[thisSel] = map[string]interface{}{}
		//}

		current = curMSI[thisSel]
	default:
		current = nil
	}
	// do we need to access the item of an array?
	//if index > -1 {
	//  if array, ok := current.([]interface{}); ok {
	//    if index < len(array) {
	//      current = array[index]
	//    } else {
	//      current = nil
	//    }
	//  }
	//}
	if len(selSegs) > 1 {
		current = deleteKey(current, selSegs[1])
	}
	return current
}

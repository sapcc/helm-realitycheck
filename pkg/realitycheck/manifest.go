package realitycheck

import (
	"fmt"
	"io"
	"log"

	"github.com/go-test/deep"
	"github.com/golang/glog"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/genericclioptions/resource"
)

func DiffManifest(config *genericclioptions.ConfigFlags, stream io.Reader, namespace string) {
	builder := resource.NewBuilder(config).
		ContinueOnError().
		Unstructured().
		NamespaceParam(namespace).
		DefaultNamespace().
		Stream(stream, "").
		Flatten()

	result := builder.Do()

	infos, err := result.Infos()
	if err != nil {
		glog.Fatal(err)
	}

	for _, info := range infos {

		object := info.Object
		gvk := object.GetObjectKind().GroupVersionKind()

		helper := resource.NewHelper(info.Client, info.Mapping)
		fromServer, err := helper.Get(info.Namespace, info.Name, false)
		if err != nil {
			log.Fatal(err)
		}

		if gvk.Kind == "OpenstackSeed" {
			glog.Infof("Create watcher for %s %s/%s", gvk.String(), info.Namespace, info.Name)
			watcher, err := helper.Watch("", "", &meta_v1.ListOptions{})
			if err != nil {
				glog.Fatal(err)
			}
			for event := range watcher.ResultChan() {
				obj := event.Object.(*unstructured.Unstructured)
				glog.Infof("type: %s, %s/%s", event.Type, obj.GetNamespace(), obj.GetName())

			}
			log.Fatal("My watch has ended")
		}

		filteredFromServer := fromServer.(*unstructured.Unstructured).Object
		unstructured.RemoveNestedField(filteredFromServer, "metadata", "selfLink")
		unstructured.RemoveNestedField(filteredFromServer, "metadata", "creationTimestamp")
		unstructured.RemoveNestedField(filteredFromServer, "metadata", "deletionTimestamp")
		unstructured.RemoveNestedField(filteredFromServer, "metadata", "clusterName")
		unstructured.RemoveNestedField(filteredFromServer, "metadata", "generation")
		unstructured.RemoveNestedField(filteredFromServer, "metadata", "uid")
		unstructured.RemoveNestedField(filteredFromServer, "metadata", "deletionGracePeriodSeconds")
		unstructured.RemoveNestedField(filteredFromServer, "metadata", "resourceVersion")

		diffs := deep.Equal(object, fromServer.(*unstructured.Unstructured))

		if len(diffs) > 0 {
			fmt.Println(info.Object.GetObjectKind().GroupVersionKind().String(), " is different")
			for _, diff := range diffs {
				fmt.Println(diff)
			}
		} else {
			fmt.Println(gvk.String(), " OK")

		}

	}
}

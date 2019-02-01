package main

import (
	"flag"
	"fmt"
	"log"
	"strings"

	"github.com/d4l3k/messagediff"
	"github.com/golang/glog"
	"github.com/sapcc/helm-realitycheck/helm"
	"github.com/spf13/pflag"
	batch_v1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	v1beta1 "k8s.io/api/extensions/v1beta1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/genericclioptions/resource"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
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

	releaseResponse, err := helmClient.ReleaseContent(pflag.Arg(0))
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
	if err != nil && len(infos) == 0 {
		log.Fatal(err)
	}

	for _, info := range infos {

		object := info.Object
		gvk := object.GetObjectKind().GroupVersionKind()

		helper := resource.NewHelper(info.Client, info.Mapping)
		fromServer, err := helper.Get(info.Namespace, info.Name, false)
		if err != nil {
			glog.Error(err)
			continue
		}
		fromServer.GetObjectKind().SetGroupVersionKind(gvk)
		objectMeta := fromServer.(meta_v1.ObjectMetaAccessor).GetObjectMeta()
		objectMeta.SetUID("")
		objectMeta.SetCreationTimestamp(meta_v1.Time{})
		objectMeta.SetSelfLink("")
		objectMeta.SetResourceVersion("")
		objectMeta.SetGeneration(0)

		switch v := object.(type) {
		case *v1.Service:
			for i, _ := range v.Spec.Ports {
				if v.Spec.Ports[i].Protocol == "" {
					v.Spec.Ports[i].Protocol = v1.ProtocolTCP
				}
				if v.Spec.Ports[i].TargetPort.String() == "0" {
					v.Spec.Ports[i].TargetPort = intstr.FromInt(int(v.Spec.Ports[i].Port))
				}
			}
			if v.Spec.Type == "" {
				v.Spec.Type = v1.ServiceTypeClusterIP
			}
			if v.Spec.SessionAffinity == "" {
				v.Spec.SessionAffinity = v1.ServiceAffinityNone
			}
			if v.Spec.ClusterIP == "" {
				fromServer.(*v1.Service).Spec.ClusterIP = ""
			}
		case *v1beta1.Deployment:
			fromAPI := fromServer.(*v1beta1.Deployment)
			fromAPI.Status = v1beta1.DeploymentStatus{}
			delete(fromAPI.Annotations, "deployment.kubernetes.io/revision")

			if v.Annotations == nil {
				v.Annotations = map[string]string{}
			}
			podSpecTemplateDefault(&v.Spec.Template)
		case *v1beta1.DaemonSet:
			fromAPI := fromServer.(*v1beta1.DaemonSet)
			fromAPI.Spec.TemplateGeneration = 0
			podSpecTemplateDefault(&v.Spec.Template)
			fromAPI.Status = v1beta1.DaemonSetStatus{}
		case *batch_v1.Job:
			fromAPI := fromServer.(*batch_v1.Job)
			fromAPI.Status = batch_v1.JobStatus{}
			fromAPI.Spec.Selector = nil
			if v.Spec.Template.Labels == nil {
				fromAPI.Spec.Template.Labels = nil
			} else {
				delete(fromAPI.Spec.Template.Labels, "controller-uid")
				delete(fromAPI.Spec.Template.Labels, "job-name")
			}
			podSpecTemplateDefault(&v.Spec.Template)
			if v.Spec.Completions == nil {
				v.Spec.Completions = &defaultOne
			}
			if v.Spec.Parallelism == nil {
				v.Spec.Parallelism = &defaultOne
			}
		case *v1beta1.Ingress:
			fromServer.(*v1beta1.Ingress).Status = v1beta1.IngressStatus{}
		}

		diff, equal := messagediff.PrettyDiff(object, fromServer)
		if equal {
			fmt.Printf("%s %s/%s OK\n", gvk.String(), info.Namespace, info.Name)
		} else {
			fmt.Printf("%s %s/%s differs!!!\n", gvk.String(), info.Namespace, info.Name)
			fmt.Println(diff)
		}
	}

	//spew.Dump(fromServer)
}
func defaultValue(ptr *string, defaultValue string) {
	if *ptr == "" {
		*ptr = defaultValue
	}
}

func probeDefaults(probe *v1.Probe) {
	if probe == nil {
		return
	}
	if probe.FailureThreshold == 0 {
		probe.FailureThreshold = 3
	}
	if probe.SuccessThreshold == 0 {
		probe.SuccessThreshold = 1
	}
	if probe.PeriodSeconds == 0 {
		probe.PeriodSeconds = 10
	}
	if probe.TimeoutSeconds == 0 {
		probe.TimeoutSeconds = 1
	}
	if probe.Handler.HTTPGet != nil && probe.Handler.HTTPGet.Scheme == "" {
		probe.Handler.HTTPGet.Scheme = v1.URISchemeHTTP
	}
}

var (
	defaultGracePeriod  = int64(v1.DefaultTerminationGracePeriodSeconds)
	cmSourceDefaultMode = v1.ConfigMapVolumeSourceDefaultMode
	defaultOne          = int32(1)
)

func podSpecTemplateDefault(template *v1.PodTemplateSpec) {
	if template.Spec.DNSPolicy == "" {
		template.Spec.DNSPolicy = v1.DNSClusterFirst
	}
	if template.Spec.RestartPolicy == "" {
		template.Spec.RestartPolicy = v1.RestartPolicyAlways
	}
	if template.Spec.SchedulerName == "" {
		template.Spec.SchedulerName = v1.DefaultSchedulerName
	}
	if template.Spec.TerminationGracePeriodSeconds == nil {
		template.Spec.TerminationGracePeriodSeconds = &defaultGracePeriod
	}
	if template.Spec.SecurityContext == nil {
		template.Spec.SecurityContext = new(v1.PodSecurityContext)
	}
	containers := template.Spec.Containers
	for i, _ := range containers {
		if containers[i].TerminationMessagePath == "" {
			containers[i].TerminationMessagePath = v1.TerminationMessagePathDefault
			containers[i].TerminationMessagePolicy = v1.TerminationMessageReadFile
		}
		probeDefaults(containers[i].LivenessProbe)
		probeDefaults(containers[i].ReadinessProbe)
		ports := containers[i].Ports
		for p, _ := range ports {
			if ports[p].Protocol == "" {
				ports[p].Protocol = v1.ProtocolTCP
			}
		}
		envs := containers[i].Env
		for e, _ := range envs {
			valueFrom := envs[e].ValueFrom
			if valueFrom != nil && valueFrom.FieldRef != nil && valueFrom.FieldRef.APIVersion == "" {
				valueFrom.FieldRef.APIVersion = "v1"
			}
		}
	}

	volumes := template.Spec.Volumes
	for v, _ := range volumes {
		if volumes[v].VolumeSource.ConfigMap != nil && volumes[v].VolumeSource.ConfigMap.DefaultMode == nil {
			volumes[v].VolumeSource.ConfigMap.DefaultMode = &cmSourceDefaultMode
		}
	}
}

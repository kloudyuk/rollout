package main

import (
	"fmt"
	"os"
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var synced = false

func init() {
	log.SetLogger(zap.New())
}

func main() {
	entryLog := log.Log.WithName("entrypoint")
	entryLog.Info("create clientset")
	clientset, err := kubernetes.NewForConfig(config.GetConfigOrDie())
	if err != nil {
		entryLog.Error(err, "failed to create clientset")
		os.Exit(1)
	}
	informerFactory := informers.NewSharedInformerFactory(clientset, 1*time.Hour)
	informerLogger := log.Log.WithName("informer")

	entryLog.Info("create Deployment informer")
	deploymentInformer := informerFactory.Apps().V1().Deployments().Informer()
	entryLog.Info("register Deployment event handlers")
	_, err = deploymentInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			handler(
				nil,
				obj.(*appsv1.Deployment),
				informerLogger,
			)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			handler(
				oldObj.(*appsv1.Deployment),
				newObj.(*appsv1.Deployment),
				informerLogger,
			)
		},
	})
	if err != nil {
		entryLog.Error(err, "failed to register Deployment event handlers")
		os.Exit(1)
	}

	entryLog.Info("create DaemonSet informer")
	daemonsetInformer := informerFactory.Apps().V1().DaemonSets().Informer()
	entryLog.Info("register DaemonSet event handlers")
	_, err = daemonsetInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			handler(
				nil,
				obj.(*appsv1.DaemonSet),
				informerLogger,
			)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			handler(
				oldObj.(*appsv1.DaemonSet),
				newObj.(*appsv1.DaemonSet),
				informerLogger,
			)
		},
	})
	if err != nil {
		entryLog.Error(err, "failed to register DaemonSet event handlers")
		os.Exit(1)
	}

	entryLog.Info("start informers")
	informerFactory.Start(wait.NeverStop)
	entryLog.Info("waiting for cache sync")
	informerFactory.WaitForCacheSync(wait.NeverStop)
	entryLog.Info("cache synced")
	synced = true
	select {}
}

func handler(old, new client.Object, log logr.Logger) {
	if !synced {
		return
	}
	var spec v1.PodSpec
	switch t := new.(type) {
	case *appsv1.Deployment:
		spec = new.(*appsv1.Deployment).Spec.Template.Spec
	case *appsv1.DaemonSet:
		spec = new.(*appsv1.DaemonSet).Spec.Template.Spec
	default:
		log.Error(nil, "invalid type", "type", t)
		return
	}
	if old == nil || new.GetGeneration() != old.GetGeneration() {
		// A deployment happened
		images := map[string]string{}
		for _, c := range spec.Containers {
			images[c.Name] = c.Image
		}
		log.WithValues("name", new.GetName(), "namespace", new.GetNamespace(), "images", images).Info("deployment", "kind", fmt.Sprintf("%T", new))
	}
}

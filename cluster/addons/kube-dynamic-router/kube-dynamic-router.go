/*
Copyright 2014 Google Inc. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// kube2sky is a bridge between Kubernetes and SkyDNS.  It watches the
// Kubernetes master for changes in Services and manifests them into etcd for
// SkyDNS to serve as DNS records.
package main

import (
	_ "encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	_ "time"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	klabels "github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	kwatch "github.com/GoogleCloudPlatform/kubernetes/pkg/watch"
)

var (
	verbose = flag.Bool("verbose", false, "log extra information")
	label   = flag.String("label", "website", "the label to match")
)

// TODO: evaluate using pkg/client/clientcmd
func newKubeClient() (*kclient.Client, error) {
	masterHost := os.Getenv("KUBERNETES_RO_SERVICE_HOST")
	if masterHost == "" {
		log.Fatalf("KUBERNETES_RO_SERVICE_HOST is not defined")
	}
	masterPort := os.Getenv("KUBERNETES_RO_SERVICE_PORT")
	if masterPort == "" {
		log.Fatalf("KUBERNETES_RO_SERVICE_PORT is not defined")
	}

	config := &kclient.Config{
		Host:    fmt.Sprintf("http://%s:%s", masterHost, masterPort),
		Version: "v1beta1",
	}
	log.Printf("Using %s for kubernetes master", config.Host)
	log.Printf("Using kubernetes API %s", config.Version)
	return kclient.New(config)
}

func watchOnce(kubeClient *kclient.Client) {
	// Start the goroutine to produce update events.
	updates := make(chan podUpdate)
	startWatching(kubeClient.Pods(kapi.NamespaceAll), updates)

	// This loop will break if the channel closes, which is how the
	// goroutine signals an error.
	for ev := range updates {
		if *verbose {
			log.Printf("Received update event: %#v", ev)
		}
		switch ev.Op {
		case SetPods, AddPod, UpdatePod:
			for i := range ev.Pods {
				p := &ev.Pods[i]
				fmt.Printf("%v: %+v", ev.Op, p)

				fmt.Printf("\n-------------\n")
				fmt.Printf("  pod ip = %s", p.Status.PodIP)

				if fqdn, ok := p.Labels["fqdn"]; ok {
					fmt.Printf("  pod fqdn = %s", fqdn)
					deleteEndpoint(fqdn)
					writeEndpoint(fqdn, p.Status.PodIP, "80")
				}
				fmt.Printf("\n-------------\n")
				// fqdn := p.Labels["fqdn"]
			}
		case RemovePod:
			for i := range ev.Pods {
				p := &ev.Pods[i]
				fmt.Printf("Remove: %+v", p)
			}
		}
	}

	reloadNginx()
	//TODO: fully resync periodically.

}

func main() {
	flag.Parse()

	kubeClient, err := newKubeClient()
	if err != nil {
		log.Fatalf("Failed to create a kubernetes client: %v", err)
	}

	// In case of error, the watch will be aborted.  At that point we just
	// retry.
	for {
		watchOnce(kubeClient)
	}
}

//FIXME: make the below part of the k8s client lib?

// podsWatcher is capable of listing and watching for changes to pods
// across ALL namespaces matching a specific label.
type podsWatcher interface {
	List(label klabels.Selector) (*kapi.PodList, error)
	Watch(label, field klabels.Selector, resourceVersion string) (kwatch.Interface, error)
}

type operation int

// These are the available operation types.
const (
	SetPods operation = iota
	AddPod
	UpdatePod
	RemovePod
)

// serviceUpdate describes an operation of services, sent on the channel.
//
// You can add or remove a single service by sending an array of size one with
// Op == AddService|RemoveService.  For setting the state of the system to a given state, just
// set Services as desired and Op to SetServices, which will reset the system
// state to that specified in this operation for this source channel. To remove
// all services, set Services to empty array and Op to SetServices
type podUpdate struct {
	Pods []kapi.Pod
	Op   operation
}

// startWatching launches a goroutine that watches for changes to services.
func startWatching(watcher podsWatcher, updates chan<- podUpdate) {
	go watchLoop(watcher, updates)
}

// watchLoop loops forever looking for changes to services.  If an error occurs
// it will close the channel and return.
func watchLoop(podWatcher podsWatcher, updates chan<- podUpdate) {
	defer close(updates)

	pods, err := podWatcher.List(klabels.OneTermEqualSelector("name", *label))
	if err != nil {
		log.Printf("Failed to load pods: %v", err)
		return
	}
	resourceVersion := pods.ResourceVersion
	updates <- podUpdate{Op: SetPods, Pods: pods.Items}

	watcher, err := podWatcher.Watch(klabels.OneTermEqualSelector("name", *label), klabels.Everything(), resourceVersion)
	if err != nil {
		log.Printf("Failed to watch for pod changes: %v", err)
		return
	}
	defer watcher.Stop()

	ch := watcher.ResultChan()
	for {
		select {
		case event, ok := <-ch:
			if !ok {
				log.Printf("watchLoop channel closed")
				return
			}

			if event.Type == kwatch.Error {
				if status, ok := event.Object.(*kapi.Status); ok {
					log.Printf("Error during watch for pods: %#v", status)
					return
				}
				log.Fatalf("Received unexpected error: %#v", event.Object)
			}

			if pod, ok := event.Object.(*kapi.Pod); ok {
				resourceVersion = pod.ResourceVersion
				sendUpdate(updates, event, pod)
				continue
			}
		}
	}
}

func sendUpdate(updates chan<- podUpdate, event kwatch.Event, pod *kapi.Pod) {
	switch event.Type {
	case kwatch.Added:
		updates <- podUpdate{Op: AddPod, Pods: []kapi.Pod{*pod}}
	case kwatch.Modified:
		updates <- podUpdate{Op: UpdatePod, Pods: []kapi.Pod{*pod}}
	case kwatch.Deleted:
		updates <- podUpdate{Op: RemovePod, Pods: []kapi.Pod{*pod}}
	default:
		log.Fatalf("Unknown event.Type: %v", event.Type)
	}
}

func writeEndpoint(fqdn string, ip string, port string) {
	f, _ := os.Create("/nginx/sites-enabled/" + fqdn)
	defer f.Close()

	endpoints := fmt.Sprintf("\tserver %s:%s;", ip, port)
	upstream := fmt.Sprintf("upstream %s {\n %s\n}", fqdn, endpoints)

	fmt.Printf(upstream)
  fmt.Fprintf(f, upstream)
}

func deleteEndpoint(fqdn string) {
	if err := os.Remove("/nginx/sites-enabled/" + fqdn); err != nil {
		fmt.Printf("%v", err)
	}

	fmt.Printf("Deleted endpoint %v", fqdn)
}

func reloadNginx() {
	exec.Command("/service/nginxreloader")
}

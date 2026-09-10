package main

import (
	"context"
	"encoding/json"
	"github.com/WassimDhib/kubernetes-cluster-autoscaler/pkg/cloud/openstack"
	"github.com/WassimDhib/kubernetes-cluster-autoscaler/pkg/cloud/openstack/handel-node-delete"
        "github.com/WassimDhib/kubernetes-cluster-autoscaler/pkg/cloud/openstack/handle-node-add"
	"github.com/WassimDhib/kubernetes-cluster-autoscaler/pkg/common/datastructures"
	"github.com/WassimDhib/kubernetes-cluster-autoscaler/pkg/common/functions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"log"
	"plugin"
	"sync"
        "time"
)

var (
        wg                  sync.WaitGroup
        cloudType           string
        modifyEventAnalyzer func(datastructures.Event, string, string, string, string, string)
        deleteEventAnalyzer func(datastructures.Event, string, string, string, string, string)
)

func checkErr(err error) {
        if err != nil {
                log.Fatal(err)
        }
}

func main() {
        cloudType = openstackinit.ReadConfig()

        if cloudType != "OpenStack" {
                pluginName := cloudType + ".so"
                plugIn, err := plugin.Open("plugin/" + pluginName)
                if err != nil {
                        log.Fatalf("[ERROR] Could not load the plugin %s in plugin directory %v", pluginName, err)
                }

                var ok bool
                modifyEventAnalyzerSymbol, err := plugIn.Lookup("ModifyEventAnalyzer")
                modifyEventAnalyzer, ok = modifyEventAnalyzerSymbol.(func(datastructures.Event, string, string, string, string, string))
                if err != nil || !ok {
                        log.Fatalf("[ERROR] Something went wrong while loading plugin %v", err)
                }

                deleteEventAnalyzerSymbol, err := plugIn.Lookup("DeleteEventAnalyzer")
                deleteEventAnalyzer, ok = deleteEventAnalyzerSymbol.(func(datastructures.Event, string, string, string, string, string))
                if err != nil || !ok {
                        log.Fatalf("[ERROR] Something went wrong while loading plugin %v", err)
                }
                log.Printf("[INFO] %s plugin loaded successfully", cloudType)
        }
        config := functions.LoadKubeConfig()
        dynamicClient, err := dynamic.NewForConfig(config)
        checkErr(err)

        log.Println("[INFO] K8s cluster auto-scaler started")
        go watchPods(dynamicClient, config)

        select {} // Keep the main function running
}

func watchPods(dynamicClient dynamic.Interface, config *rest.Config) {
        resource := schema.GroupVersionResource{Version: "v1", Resource: "pods"}

        for {
                log.Printf("[INFO] Starting watch on pods")
                w, err := dynamicClient.Resource(resource).Namespace("").Watch(context.TODO(), metav1.ListOptions{})
                if err != nil {
                        log.Printf("[ERROR] Failed to start watch: %v", err)
                        log.Printf("[INFO] Retrying to start watch after delay")
                        time.Sleep(5 * time.Second) // Wait before retrying
                        continue
                }
                defer w.Stop()

                wg.Add(1)
                wCh := w.ResultChan()

                log.Printf("[INFO] Watcher initialized successfully")
                for event := range wCh {
                        eventFilter(event, config)
                }

                // If we exit the loop, it means the channel was closed, and we need to restart the watch
                log.Printf("[INFO] Watch channel closed, restarting watch")
                wg.Done()
        }
}

func eventFilter(event watch.Event, config *rest.Config) {
        defer handlenodeadd.PanicRecovery()

        b, _ := json.Marshal(event)
        var EventList datastructures.Event
        err := json.Unmarshal(b, &EventList)
        if err != nil {
                log.Printf("[ERROR] Failed to unmarshal event: %v", err)
                return
        }

        switch EventList.Type {
        case "MODIFIED":
                if cloudType == "OpenStack" {
                        handlenodeadd.ModifyEventAnalyzer(EventList, config)
                } else {
                        modifyEventAnalyzer(EventList, openstackinit.ProjectName, openstackinit.ClientSecret, openstackinit.ClientID, openstackinit.AWSRegion, openstackinit.AuthFile)
                }
        case "DELETED":
                if cloudType == "OpenStack" {
                        handelnodedelete.DeleteEventAnalyzer(EventList, config)
                } else {
                        deleteEventAnalyzer(EventList, openstackinit.ProjectName, openstackinit.ClientSecret, openstackinit.ClientID, openstackinit.AWSRegion, openstackinit.AuthFile)
                }
        }
}

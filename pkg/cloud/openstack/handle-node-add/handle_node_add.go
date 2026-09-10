package handlenodeadd

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"
	"math/rand"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	"github.com/gophercloud/utils/openstack/compute/v2/flavors"
	"github.com/gophercloud/utils/openstack/imageservice/v2/images"
	"github.com/gophercloud/utils/openstack/networking/v2/networks"
	"github.com/gophercloud/utils/openstack/networking/v2/extensions/security/groups"
		v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"github.com/WassimDhib/kubernetes-cluster-autoscaler/pkg/cloud/openstack"
	"github.com/WassimDhib/kubernetes-cluster-autoscaler/pkg/common/datastructures"

)

var (
	triggerLock    bool
	wg             sync.WaitGroup
	pendingPodList []string
	NetworkUUID_a  string
	NetworkUUID_d  string
	NetworkUUID_p  string
)

// IsNeededPendingStatus checks if a pod is in a pending state due to resource insufficiency.
func IsNeededPendingStatus(status v1.PodCondition) bool {
	return strings.Contains(status.Message, "Insufficient") &&
		(strings.Contains(status.Message, "cpu") || strings.Contains(status.Message, "memory")) &&
		!strings.Contains(status.Message, "had taint {node.kubernetes.io/not-ready: }, that the pod didn't tolerate")
}

// ModifyEventAnalyzer analyzes Kubernetes events to capture pending states.
func ModifyEventAnalyzer(EventList datastructures.Event, config *rest.Config) {
	status := EventList.Object.Status.Conditions[0]
	if EventList.Object.Status.Phase == "Pending" && status.Type == "PodScheduled" && status.Status == "False" {
		if IsNeededPendingStatus(status) {
			log.Printf("[ERROR] %s - %s", status.Reason, status.Message)
			wg.Add(1)
			go TriggerStatusCheck(EventList.Object, config)
		}
	}

	if EventList.Object.Status.Phase == "Pending" && (len(pendingPodList) > 0 || pendingPodList != nil) {
		PodStatus(EventList.Object)
	}
}

// TriggerStatusCheck triggers the addition of a new Kubernetes worker node.
func TriggerStatusCheck(pod v1.Pod, config *rest.Config) {
	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Println(err)
		return
	}

	nodes, err := clientSet.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Println(err)
		return
	}
	nodeCount := len(nodes.Items)

	if !triggerLock && nodeCount < openstackinit.MaxNodeCount {
		log.Println("[INFO] Node add trigger.")
		triggerLock = true
		if PendingPodListCheck(pod.Name) {
			pendingPodList = append(pendingPodList, pod.Name)
		}
		TriggerAddNode(GetOpenstackFlavor(pod))
	} else {
		if nodeCount == openstackinit.MaxNodeCount {
			log.Println("[INFO] Max node count reached")
		} else if PendingPodListCheck(pod.Name) {
			log.Println("[INFO] Node add triggered. Waiting for new node")
			pendingPodList = append(pendingPodList, pod.Name)
		}
		wg.Done()
	}
}

// PodStatus checks the status of the pending pod which triggers the new node addition process.
func PodStatus(pod v1.Pod) {
	conditions := pod.Status.Conditions[0]

	if triggerLock {
		for i, pendingPodName := range pendingPodList {
			if pod.Name == pendingPodName && conditions.Type == "PodScheduled" && conditions.Status == "True" {
				log.Printf("[INFO] %s pod scheduled.", pendingPodName)
				if len(pendingPodList) == 1 {
					pendingPodList = nil
				} else {
					pendingPodList = append(pendingPodList[:i], pendingPodList[i+1:]...)
				}
				triggerLock = false
			}
		}
	}
}

// PendingPodListCheck checks for multiple node add triggers from the same pending pod.
func PendingPodListCheck(podName string) bool {
	for _, pendingPodName := range pendingPodList {
		if pendingPodName == podName {
			return false
		}
	}
	return true
}

// GetOpenstackFlavor selects a flavor from the list of user-defined flavors.
func GetOpenstackFlavor(pod v1.Pod) string {
	defer PanicRecovery()
	var requestsCPU, requestsMemory int64
	var flavorFound bool
	index := -1

	for _, container := range pod.Spec.Containers {
		requestsCPU += container.Resources.Requests.Cpu().Value()
		requestsMemory += container.Resources.Requests.Memory().Value()
	}
	requestsMemory = requestsMemory / 1024 / 1000

	if requestsCPU != 0 || requestsMemory != 0 {
		for i, flavor := range openstackinit.FlavorsList.Flavor {
			if flavor.RequestsCPU >= requestsCPU && flavor.RequestsMemory >= requestsMemory {
				flavorFound = true
				index = i
				break
			}
		}
	}

	if index != -1 && flavorFound {
		log.Printf("[INFO] %s flavor profile selected", openstackinit.FlavorsList.Flavor[index].Name)
		return openstackinit.FlavorsList.Flavor[index].Name
	} else if requestsCPU != 0 && requestsMemory != 0 {
		panic("[ERROR] No flavor profile found")
	}

	log.Printf("[INFO] Default flavor profile %s selected", openstackinit.FlavorsList.FlavorDefault)
	return openstackinit.FlavorsList.FlavorDefault
}

// GetNodeName generates a random name for the Kubernetes worker node.
func GetNodeName() string {
	rand.Seed(time.Now().UnixNano())
	chars := []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZ" + "abcdefghijklmnopqrstuvwxyz" + "0123456789")
	length := 4
	var b strings.Builder
	for i := 0; i < length; i++ {
		b.WriteRune(chars[rand.Intn(len(chars))])
	}
	str := b.String()
	return openstackinit.PlatformPrefix + "kube-worker-" + str
}

// TriggerAddNode configures and creates a new OpenStack VM for the Kubernetes worker node.
func TriggerAddNode(flavorName string) {
	defer PanicRecovery()
	client := openstackinit.GetOpenstackToken()
	clientNeutron := openstackinit.GetOpenstackNeutronToken()
	imageID, err := images.IDFromName(client, openstackinit.ImageName)
	checkErr(err)
	flavorID, err := flavors.IDFromName(client, flavorName)
	checkErr(err)
	NodeName := GetNodeName()
	userData := `#!/usr/bin/env bash
	curl -L -s ` + openstackinit.RepoBaseUrl + `/install.sh | sudo bash -s -- \
		-i init
	`
	networkAdminName := openstackinit.NetworkAdmin.Name
	NetworkUUID_a, err := networks.IDFromName(clientNeutron, openstackinit.NetworkAdmin.Name)
	NetworkUUID_d, err := networks.IDFromName(clientNeutron, openstackinit.NetworkData.Name)
	NetworkUUID_p, err := networks.IDFromName(clientNeutron, openstackinit.NetworkPub.Name)
	OpenBar, err := groups.IDFromName(clientNeutron, openstackinit.Openbar_SG)
	log.Printf("[INFO] Resolved network admin=%q (%s), data=%s, pub=%s, security group=%s", networkAdminName, NetworkUUID_a, NetworkUUID_d, NetworkUUID_p, OpenBar)
	log.Printf("[INFO] Creating new node %s (image=%s, flavor=%s)", NodeName, imageID, flavorID)
	serverCreateOpts := servers.CreateOpts{
		Name:      NodeName,
		FlavorRef: flavorID,
		ImageRef:  imageID,
		Networks:  []servers.Network{{UUID: NetworkUUID_a}, {UUID: NetworkUUID_d}, {UUID: NetworkUUID_p}},
		SecurityGroups: []string{OpenBar},
		UserData:  []byte(userData),
	}
	server, err := servers.Create(client, serverCreateOpts).Extract()
	checkErr(err)
	log.Printf("[INFO] New node added. Node ID - %s", server.ID)
	log.Printf("[INFO] Proceeding to SGs creation for openstack instance - %s", server.ID)

	var adm1, adm2 string

	// Retrieve security group IDs for each category
	adm1, err = groups.IDFromName(clientNeutron, openstackinit.NetworkAdmin.SecurityGroups[0])
	checkErr(err)
	adm2, err = groups.IDFromName(clientNeutron, openstackinit.NetworkAdmin.SecurityGroups[1])
	checkErr(err)
	log.Printf("[INFO] Resolved admin security groups: %s=%s, %s=%s", openstackinit.NetworkAdmin.SecurityGroups[0], adm1, openstackinit.NetworkAdmin.SecurityGroups[1], adm2)

	

	// portsCreateOpts := []ports.CreateOpts{
	// 	{
	// 		NetworkID:      NetworkUUID_a,
	// 		SecurityGroups: &[]string{adm1, adm2},
	// 		DeviceID: server.ID,
	// 	},
	// }


	// for _, createOpts := range portsCreateOpts {
	// 	port, err := ports.Create(clientNeutron, createOpts).Extract()
	// 	if err != nil {
	// 		log.Fatalf("Error creating port: %v", err)
	// 	}
	// 	log.Printf("Created port: %+v", port)
	// }
	
	
	NewNodeStatus(server.ID)
}

// NewNodeStatus checks the status of the new node.
func NewNodeStatus(id string) {
	log.Println("[INFO] Checking node status")
	ready, err := NewNodeReady(id)
	if err != nil {
		log.Printf("[ERROR] Error creating the server %s", err)
		return
	}
	if ready {
		log.Println("[INFO] Node is running.")
	}
	wg.Done()
}

// NewNodeReady continuously checks if the new node is ready and active.
func NewNodeReady(id string) (bool, error) {
	client := openstackinit.GetOpenstackToken()

	for {
		server, err := servers.Get(client, id).Extract()
		if err != nil {
			return false, err
		}

		if server.Status == "ACTIVE" {
			return true, nil
		}
		time.Sleep(10 * time.Second) // Pause to prevent tight loop
	}
}

// PanicRecovery recovers from panics to ensure application stability.
func PanicRecovery() {
	if r := recover(); r != nil {
		log.Println("[ERROR]", r)
		triggerLock = false // Reset lock to prevent deadlock
	}
}

// checkErr is a helper to handle errors succinctly.
func checkErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

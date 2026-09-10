# This is a sample plugin structure.

This code sample use AWS Go SDK to provide an idea about how to write a plugin for Kubernetes cluster auto scalar.

Plugin should have `ModifyEventAnalyzer` and `DeleteEventAnalyzer` main function. Main `autoscalar` will search for these two functions in the plugin.

Build the plugin,
```
go build -buildmode=plugin -o AWS.so main.go
```

Read more about AWS Go SDK here,
[AWS SDK for Go Developer Guide](https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/welcome.html)

[AWS SDK for Go API Reference](https://docs.aws.amazon.com/sdk-for-go/api/)



## Procedure Steps: 
### 1. Modifier le code sur forge, attention à la branche au Dockerfile 
### 2. pour builder passer sur ton compte . 
### 3. pour déplacer vers le remote registry, il faut passer sur le compte jtorkhani: 
A. 
```
sudo docker save registry-dev.example-internal.local/autoscaler:v1.0.18-n6 -o autopscaler6.tar.gz
```
B. 
```
sudo chown jtorkhani: autopscaler6.tar.gz 
```
C. lancer la commande sans mode root: 
```
	jtorkhani$ scp autopscaler6.tar.gz as-remote-registry:/tmp
```
D. en tant que jtorkhani, accèder au registy par : 
```
ssh as-remote-registry
```
E. Passer en root, et faire un load de l'image a partir du tar sous /tmp: 
```
	docker load < /tmp/autopscaler6.tar.gz
```
	F. changer de tag, supprim l'intérim, et pousser le new tag: 
```
	docker tag  registry-dev.example-internal.local/autoscaler:v1.0.18-n6  registry.example-internal.local/autoscaler:v1.0.18-n6
	docker image rm registry-dev.example-internal.local/autoscaler:v1.0.18-n6
	docker push registry.example-internal.local/autoscaler:v1.0.18-n6
```	
4. une fois l'image est prête, KUBERNETES TIME: 
Accès: pour accèder fair un ssh as-remote-server depuis le compte jtorkhani. 
Dans: /home/cloudadm/kubernetes-cluster-autoscaler-master/deployment il y'a des exemplaires des configmaps par itération de container.

Pour editer le configmap actif: 
```
kubectl edit cm -n kube-system autoscalar-config 
```
Sinon par kubectl apply -f .  

Supprimez le pod autoscaler Running pour prendre en charge les modifs sur la cm: 
```
Kubectl delete pod autopscaler-pod-name
```
Pour utiliser le nouveau autoscaler container: 
```
kubectl edit deploy autoscalar -n kube-system  
```
 et mettre ajour le tag (cherchez par /image )   
Pour déclencher un scaling: 
```
 kubectl edit deploy -n mce appli-sabredav-mce-deployment
```
Quand le Vim ouvre, cherchez replicas et monter le nombre. 

Trackez les events autoscaler par: 
```
kubectl logs -f autopscaler-pod-name
```
```
cloudadm] $ kubectl logs -f  autoscalar-999bb9b99-h6hwf 
2024/11/26 16:44:52 [INFO] K8s cluster auto-scaler started edit wdh 191124
2024/11/26 16:44:52 [INFO] Starting watch on pods
2024/11/26 16:44:52 [INFO] Watcher initialized successfully
2024/11/26 16:47:41 [ERROR] Unschedulable - 0/7 nodes are available: 1 node(s) didn't match Pod's node affinity/selector, 3 Insufficient memory, 3 node(s) had untolerated taint {node-role.kubernetes.io/control-plane: }. preemption: 0/7 nodes are available: 3 No preemption victims found for incoming pod, 4 Preemption is not helpful for scheduling.
2024/11/26 16:47:41 [ERROR] Unschedulable - 0/7 nodes are available: 1 node(s) didn't match Pod's node affinity/selector, 3 Insufficient memory, 3 node(s) had untolerated taint {node-role.kubernetes.io/control-plane: }. preemption: 0/7 nodes are available: 3 No preemption victims found for incoming pod, 4 Preemption is not helpful for scheduling.
2024/11/26 16:47:41 [INFO] Node add trigger.
2024/11/26 16:47:41 [INFO] CO1.4 flavor profile selected
2024/11/26 16:47:41 [ERROR] Unschedulable - 0/7 nodes are available: 1 node(s) didn't match Pod's node affinity/selector, 3 Insufficient memory, 3 node(s) had untolerated taint {node-role.kubernetes.io/control-plane: }. preemption: 0/7 nodes are available: 3 No preemption victims found for incoming pod, 4 Preemption is not helpful for scheduling.
2024/11/26 16:47:41 [INFO] Node add triggered. Waiting for new node
2024/11/26 16:47:41 [INFO] Node add triggered. Waiting for new node
2024/11/26 16:47:43 [INFO] Creating new node with Node_Name = scaled-epocv2-qua01-kube-worker-CJ0T, imageId = b0921498-d12a-440a-9052-8771a0d8f083, flavorID = 39679b0d-1df2-49b8-83e0-81f517061403
2024/11/26 16:47:44 [INFO] New node added. Node ID - 4237e03c-7b87-4e96-9621-c4fa3cff1e07
2024/11/26 16:47:45 Resource not found: [PUT https://openstack.example-internal.local:9696/v2.0/ports/], error message: {"NeutronError": {"message": "Port None could not be found.", "type": "PortNotFound", "detail": ""}}


```
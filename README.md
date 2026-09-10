## Kubernetes Cluster Autoscaler

A go app that correlates kubernetes events related to POD resources, and provisions an OpenStack intance as a kubernetes extra node. 

### Autoscaler authentication config:
The autoscaler needs to authenticate to kubernetes (kubeconfig passed from secret) and openstack ( Config.yml passed from configmap.) to function.  
### Iteration of config.yml:
Be advised that there is no generic structure of this config file, the project is in active development and thus the file is prone to upadates along the way.
```
apiVersion: v1
kind: ConfigMap
metadata:
  name: autoscalar-config
  namespace: kube-system
data:
  conf.yml: |
    # OpenStack, GCP, AWS, libvirt, Other
    CloudType: OpenStack

    #  Other Supported Keys accordign to the CloudType chose.
    # AuthOptions:
    #   ProjectName: ""
    #   ClientSecret: ""
    #   ClientId: ""
    #   AWSRegion: ""
    #   AuthFile: ""
    AuthOptions:
      IdentityEndpoint: "https://identity.example-internal.local:5000"
      Username: "Redacted"
      Password: "Redacted"
      TenantID: "Redacted"
      DomainName: ""

    # Common for any CloudType Select
    WorkerImageName: ""
    # Cool Down Time in seconds
    CoolDownTime: 600
    # Minimum number of nodes in the Cluster including master node. 2 equals to one master and one worker.
    MinNodeCount: 2
    # Minimum number of nodes in the Cluster
    MaxNodeCount: 5
    PassConfigToPlugin: false

    Network:
      SecurityGroupName: ""
      NetworkUUID: ""

    OpenStackFlavours:
      DefaultFlavour: "t2.medium"
      Flavours:
      - Name: "t2.medium"
        VCPU: 2
        Memory: 4096
      - Name: "t2.large"
        VCPU: 2
        Memory: 8192

```

The kubeconfig file is passed via a secret and mounted to `~/.kube/kubeconfig` using the command: 
```
kubectl create secret generic my-kubeconfig-secret --from-file=kubeconfig=/path/to/kubeconfig
```
## Building an Autoscale docker image: 
on ``dockerfiles/Alpine`` there's a bash script that reads defaults from an .env (added to gitignore for security purposes).  
The script needs to read the following variables from a .env file such as :  
```
# .env file

# Default target for SCP
# Change this if needed or leave it to use the default defined in the script
as_remote_registry=as-remote-registry

# Git credentials (make sure to keep this information secure)
gituser=your_git_username
gitpass=your_git_password

```
The script is ran with :
```
./image-build.sh <tag_name>

```
Example: 
```
./image-build.sh v1.0.18-n7
```


## Roadmap 

- Fix watchers going idle after a certain period of inactivity ✅ 
- Fix watchers inability to detect pods events prior to the autoscaler deployment ✅ 
- Variablize networks, and Given That networks IDs are prone to change, Re-integrate IDFROMNAME func logic ✅
- Containerd - Flannel network troubleshooting ✅


### Detailed specs on: 
#### Spread consolidated Security Groups into respective Networks  ⏳
This feature requires deleting the PORTS ALLOCATION on Openstack (due to quota limitation) when freeing the scaled VM.
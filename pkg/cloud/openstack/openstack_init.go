package openstackinit

import (
	"github.com/WassimDhib/kubernetes-cluster-autoscaler/pkg/common/datastructures"
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"log"
	"time"
	"os"
	"regexp"
)


// FlavorsList, and other list og global variables
var (
	FlavorsList         datastructures.FlavorList
	CoolDownTime        time.Duration
	IgnoreNamespaceList map[string]bool
	MinNodeCount        int
	MaxNodeCount        int
	ImageName           string
	PlatformPrefix      string
	RepoBaseUrl         string
	IdentityEndpoint    string
	Username            string
	Password            string
	TenantID            string
	DomainName          string
	ProjectName         string
	ClientSecret        string
	ClientID            string
	AWSRegion           string
	AuthFile            string
	Networks 			[]Network
	NetworkAdmin 		Network
	NetworkData 		Network
	SecurityGroupsMap 	map[string][]string
	NetworkPub 			Network
	Openbar_SG 			string
)

// ConfigYaml used to decode the configuration file
type ConfigYaml struct {
	CloudType          string            `yaml:"CloudType"`
	AuthOptions        AuthOptions       `yaml:"AuthOptions"`
	Networks 		   []Network 		 `yaml:"Networks"` // Map structure for networks
	WorkerImageName    string            `yaml:"WorkerImageName"`
	PlatformPrefix	   string            `yaml:"PlatformPrefix"`
	RepoBaseUrl        string            `yaml:"RepoBaseUrl"`
	CoolDownTime       int               `yaml:"CoolDownTime"`
	MinNodeCount       int               `yaml:"MinNodeCount"`
	MaxNodeCount       int               `yaml:"MaxNodeCount"`
	OpenStackFlavours  OpenStackFlavours `yaml:"OpenStackFlavours"`
	PassConfigToPlugin bool              `yaml:"PassConfigToPlugin"`
	Openbar_SG         string			 `yaml:"Openbar_SG"`	
}

// AuthOptions list of credentials to authenticate cloud infrastructure
type AuthOptions struct {
	IdentityEndpoint string `yaml:"IdentityEndpoint"`
	Username         string `yaml:"Username"`
	Password         string `yaml:"Password"`
	TenantID         string `yaml:"TenantID"`
	DomainName       string `yaml:"DomainName"`
	ProjectName      string `yaml:"ProjectName"`
	ClientSecret     string `yaml:"ClientSecret"`
	ClientID         string `yaml:"ClientId"`
	AWSRegion        string `yaml:"AWSRegion"`
	AuthFile         string `yaml:"AuthFile"`
}

// Network OpenStack network configuration to used
// when creating worker nodes
type Network struct {
	Name 		  string   `yaml:"Name"`
	UUID          string   `yaml:"UUID"`
	Port          string   `yaml:"port,omitempty"`
	FixedIP       string   `yaml:"fixed_ip,omitempty"`
	Subnet        string   `yaml:"subnet,omitempty"`  
	SecurityGroups []string `yaml:"SecurityGroups"`
	Tags          []string `yaml:"tags,omitempty"`    
	Check         bool     `yaml:"check,omitempty"`   
}



// OpenStackFlavours user configured Open Stack Flavours in the config file.
type OpenStackFlavours struct {
	DefaultFlavour string     `yaml:"DefaultFlavour"`
	Flavours       []Flavours `yaml:"Flavours"`
}

// Flavours configured in config.yml
type Flavours struct {
	Name   string `yaml:"Name"`
	VCPU   int64  `yaml:"VCPU"`
	Memory int64  `yaml:"Memory"`
}

// ReadConfig read and configure starup variables from the config.yml
func ReadConfig() string {
	ConfigFile, err := ioutil.ReadFile("conf.yml")
	if err != nil {
		log.Fatalf("[ERROR] Error reading Config YAML file: %s\n", err)
	}

	conf := ConfigYaml{}
	err = yaml.Unmarshal(ConfigFile, &conf)
	if err != nil {
		log.Fatalf("[ERROR] Error decoding Config YAML file: %s\n", err)
	}

	if conf.CloudType == "" {
		log.Fatal("[ERROR] \"CloudType\" must be set to one of OpenStack, GCP, AWS, libvirt, Other value.")
	}
	IdentityEndpoint = conf.AuthOptions.IdentityEndpoint
	Username = conf.AuthOptions.Username
	Password = conf.AuthOptions.Password
	TenantID = conf.AuthOptions.TenantID
	DomainName = conf.AuthOptions.DomainName
	Openbar_SG = conf.Openbar_SG
	admRegex := regexp.MustCompile(`adm`)
	dataRegex := regexp.MustCompile(`data`)
	pubRegex := regexp.MustCompile(`pub`)

	for _, network := range conf.Networks {
		switch {
		case admRegex.MatchString(network.Name):
			NetworkAdmin = network
		case dataRegex.MatchString(network.Name):
			NetworkData = network
		case pubRegex.MatchString(network.Name):
			NetworkPub = network
		}
	}
	
	CoolDownTime = time.Duration(conf.CoolDownTime)
	MinNodeCount = conf.MinNodeCount
	MaxNodeCount = conf.MaxNodeCount
	ImageName = conf.WorkerImageName
	PlatformPrefix = conf.PlatformPrefix
	RepoBaseUrl = conf.RepoBaseUrl
	// Assign network configurations to specific variables based on their keys
	// Iterate over the slice to assign networks to respective variables
	SecurityGroupsMap = make(map[string][]string)
	for _, network := range conf.Networks {
		SecurityGroupsMap[network.Name] = network.SecurityGroups
	}

	var FlavorDetails []datastructures.FlavorDetails
	for _, Flavor := range conf.OpenStackFlavours.Flavours {
		FlavorDetails = append(FlavorDetails, datastructures.FlavorDetails{Flavor.Name, Flavor.VCPU, Flavor.Memory})
	}

	FlavorsList = datastructures.FlavorList{len(conf.OpenStackFlavours.Flavours), FlavorDetails, conf.OpenStackFlavours.DefaultFlavour}
	IgnoreNamespaceList = map[string]bool{"ingress-nginx": true, "kube-node-lease": true, "kube-public": true, "kube-system": true}

	return conf.CloudType
}

// GetOpenstackToken authenticate OpenStack cloud
func GetOpenstackToken() *gophercloud.ServiceClient {
	
	opts := gophercloud.AuthOptions{
		IdentityEndpoint: IdentityEndpoint,
		Username:         Username,
		Password:         Password,
		TenantID:         TenantID,
		DomainName:       DomainName,
	}

	provider, err := openstack.AuthenticatedClient(opts)
 
	if err != nil {
		panic(err)
	}
	client, err := openstack.NewComputeV2(provider, gophercloud.EndpointOpts{Region: os.Getenv("OS_REGION_NAME")})
	if err != nil {
		panic(err)
	}
 
	return client
}



// GetOpenstackNeutronToken authenticate OpenStack cloud for Neutron
func GetOpenstackNeutronToken() *gophercloud.ServiceClient {
	
	opts := gophercloud.AuthOptions{
		IdentityEndpoint: IdentityEndpoint,
		Username:         Username,
		Password:         Password,
		TenantID:         TenantID,
		DomainName:       DomainName,
	}

	provider, err := openstack.AuthenticatedClient(opts)
 
	if err != nil {
		panic(err)
	}
	client, err := openstack.NewNetworkV2(provider, gophercloud.EndpointOpts{Region: os.Getenv("OS_REGION_NAME")})
	if err != nil {
		panic(err)
	}
 
	return client
}
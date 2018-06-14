package main 

import (
	"encoding/json"
	"os"
	"time"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	cert "crypto/x509"
	"crypto/tls"
	"context"
	"gopkg.in/yaml.v2"
	"github.com/coreos/etcd/client"
)

type composition struct {
	Kind string `yaml:"kind"`
	Plural string `yaml:"plural"`
	Endpoint string `yaml:"endpoint"`
	Composition []string `yaml:"composition"`
}

type MetaDataAndOwnerReferences struct {
	MetaDataName string
	OwnerReferenceName string
	OwnerReferenceKind string
	OwnerReferenceAPIVersion string
}

type CompositionTreeNode struct {
	Level int
	ChildKind string
	Children []MetaDataAndOwnerReferences
}

type Provenance struct {
	Kind string
	Name string
	CompositionTree *[]CompositionTreeNode
}

var (
	serviceHost string
	servicePort string
	namespace string
	httpMethod string
	etcdServiceURL string

	kindPluralMap map[string]string
	kindVersionMap map[string]string
	compositionMap map[string][]string

	REPLICA_SET string
	DEPLOYMENT string
	POD string
	CONFIG_MAP string
	SERVICE string
	ETCD_CLUSTER string

	clusterProvenance []Provenance
)

func init() {
	clusterProvenance = []Provenance{}
	serviceHost = os.Getenv("KUBERNETES_SERVICE_HOST")
	servicePort = os.Getenv("KUBERNETES_SERVICE_PORT")
	namespace = "default"
	httpMethod = http.MethodGet

	etcdServiceURL = "http://example-etcd-cluster-client:2379"

	DEPLOYMENT = "Deployment"
	REPLICA_SET = "ReplicaSet"
	POD = "Pod"
	CONFIG_MAP = "ConfigMap"
	SERVICE = "Service"
	ETCD_CLUSTER = "EtcdCluster"

	kindPluralMap = make(map[string]string)
	kindVersionMap = make(map[string]string)
	compositionMap = make(map[string][]string,0)

	/*
	kindPluralMap[DEPLOYMENT] = "deployments"
	kindPluralMap[REPLICA_SET] = "replicasets"
	kindPluralMap[POD] = "pods"
	kindPluralMap[CONFIG_MAP] = "configmaps"
	kindPluralMap[SERVICE] = "services"
	kindPluralMap[ETCD_CLUSTER] = "etcdclusters"

	kindVersionMap[DEPLOYMENT] = "apis/apps/v1"
	kindVersionMap[REPLICA_SET] = "apis/extensions/v1beta1"
	kindVersionMap[POD] = "api/v1"
	kindVersionMap[CONFIG_MAP] = "api/v1"
	kindVersionMap[SERVICE] = "api/v1"
	kindVersionMap[ETCD_CLUSTER] = "apis/etcd.database.coreos.com/v1beta2"

	compositionMap[DEPLOYMENT] = []string{REPLICA_SET}
	compositionMap[REPLICA_SET] = []string{POD}
	compositionMap[ETCD_CLUSTER] = []string{POD, SERVICE}
	*/
}

// Reference: 
// 1. https://stackoverflow.com/questions/30690186/how-do-i-access-the-kubernetes-api-from-within-a-pod-container
// 2. https://www.sohamkamani.com/blog/2017/10/18/parsing-json-in-golang/#unstructured-data
// 3. https://github.com/coreos/etcd/tree/master/client

func main() {

	for {
		readKindCompositionFile()
		provenanceToPrint := false
		resourceKindList := getResourceKinds()
		for _, resourceKind := range resourceKindList {
			resourceNameList := getResourceNames(resourceKind)
			for _, resourceName := range resourceNameList {
				provenanceNeeded := checkIfProvenanceNeeded(resourceKind, resourceName)
				if provenanceNeeded {
					fmt.Println("###################################")
					fmt.Printf("Building Provenance for %s %s\n", resourceKind, resourceName)
					level := 1
					compositionTree := []CompositionTreeNode{}
					buildProvenance(resourceKind, resourceName, level, &compositionTree)
					storeProvenance(resourceKind, resourceName, &compositionTree)
					fmt.Println("###################################\n")
					provenanceToPrint = true
				}
			}
		}
		if provenanceToPrint {
			printProvenance()
		}
		time.Sleep(time.Second * 5)
	}
}

func checkIfProvenanceNeeded(resourceKind, resourceName string) bool {
	for _, provenanceItem := range clusterProvenance {
		kind := provenanceItem.Kind
		name := provenanceItem.Name
		if resourceKind == kind && resourceName == name {
			return false
		}
	}
	return true
}

func readKindCompositionFile() {
	// read from the opt file
    filePath := os.Getenv("KIND_COMPOSITION_FILE")
    //filePath := "./kind_compositions.yaml"
    yamlFile, err := ioutil.ReadFile(filePath)
    if err != nil {
    	fmt.Printf("Error reading file:%s", err)
    }

    compositionsList := make([]composition,0)
    err = yaml.Unmarshal(yamlFile, &compositionsList)

    for _, compositionObj := range compositionsList {
    	kind := compositionObj.Kind
    	endpoint := compositionObj.Endpoint
    	composition := compositionObj.Composition
    	plural := compositionObj.Plural
    	//fmt.Printf("Kind:%s, Plural: %s Endpoint:%s, Composition:%s\n", kind, plural, endpoint, composition)

    	kindPluralMap[kind] = plural
    	kindVersionMap[kind] = endpoint
    	compositionMap[kind] = composition
    }
    //printMaps()
}

func printMaps() {
	fmt.Println("Printing kindVersionMap")
	for key, value := range kindVersionMap {
		fmt.Printf("%s, %s\n", key, value)
	}
	fmt.Println("Printing kindPluralMap")
	for key, value := range kindPluralMap {
		fmt.Printf("%s, %s\n", key, value)
	}
	fmt.Println("Printing compositionMap")
	for key, value := range compositionMap {
		fmt.Printf("%s, %s\n", key, value)
	}
}

func getResourceKinds() []string {
	resourceKindSlice := make([]string, 0)
//	resourceKindSlice = append(resourceKindSlice, ETCD_CLUSTER)
//	resourceKindSlice = append(resourceKindSlice, DEPLOYMENT)
//	resourceKindSlice = append(resourceKindSlice, CONFIG_MAP)
	for key, _ := range compositionMap {
		resourceKindSlice = append(resourceKindSlice, key)
	}
	return resourceKindSlice
}

func getResourceNames(resourceKind string) []string{
	resourceApiVersion := kindVersionMap[resourceKind]
	resourceKindPlural := kindPluralMap[resourceKind]
	content := getResourceListContent(resourceApiVersion, resourceKindPlural)
	metaDataAndOwnerReferenceList := parseMetaData(content)

	var resourceNameSlice []string
	resourceNameSlice = make([]string, 0)
	for _, metaDataRef := range metaDataAndOwnerReferenceList {
		//fmt.Printf("%s\n", metaDataRef.MetaDataName)
		resourceNameSlice = append(resourceNameSlice, metaDataRef.MetaDataName)
	}
	return resourceNameSlice
}

func printProvenance() {
	fmt.Println("Provenance of different Kinds in this Cluster")
		for _, provenanceItem := range clusterProvenance {
			kind := provenanceItem.Kind
			name := provenanceItem.Name
			compositionTree := provenanceItem.CompositionTree
			fmt.Printf("Kind: %s Name: %s Composition:\n", kind, name)
			for _, compositionTreeNode := range *compositionTree {
				level := compositionTreeNode.Level
				childKind := compositionTreeNode.ChildKind
				metaDataAndOwnerReferences := compositionTreeNode.Children
				for _, metaDataNode := range metaDataAndOwnerReferences {
					childName := metaDataNode.MetaDataName
					fmt.Printf("  %d %s %s\n", level, childKind, childName)
				}
			}
			fmt.Println("============================================")
		}
}

// This stores Provenance information in memory. The provenance information will be lost
// when this Pod is deleted. 
func storeProvenance(resourceKind string, resourceName string, compositionTree *[]CompositionTreeNode) {
	provenance := Provenance{
		Kind: resourceKind,
		Name: resourceName,
		CompositionTree: compositionTree,
	}
	clusterProvenance = append(clusterProvenance, provenance)
}

// This stores Provenance information in etcd accessible at the etcdServiceURL
// One option to deploy etcd is to use the CoreOS etcd-operator.
// The etcdServiceURL initialized in init() is for the example etcd cluster that
// will be created by the etcd-operator. See https://github.com/coreos/etcd-operator
func storeProvenance_etcd(resourceKind string, resourceName string, compositionTree *[]CompositionTreeNode) {
	//fmt.Println("Entering storeProvenance")
    jsonCompositionTree, err := json.Marshal(compositionTree)
    if err != nil {
        panic (err)
    }
    resourceProv := string(jsonCompositionTree)
	cfg := client.Config{
		//Endpoints: []string{"http://192.168.99.100:32379"},
		Endpoints: []string{etcdServiceURL},
		Transport: client.DefaultTransport,
		// set timeout per request to fail fast when the target endpoint is unavailable
		//HeaderTimeoutPerRequest: time.Second,
	}
	//fmt.Printf("%v\n", cfg)
	c, err := client.New(cfg)
	if err != nil {
		log.Fatal(err)
	}
	kapi := client.NewKeysAPI(c)
	// set "/foo" key with "bar" value
	//resourceKey := "/compositions/Deployment/pod42test-deployment"
	//resourceProv := "{1 ReplicaSet; 2 Pod -1}"
	resourceKey := string("/compositions/" + resourceKind + "/" + resourceName)
	fmt.Printf("Setting %s->%s\n",resourceKey, resourceProv)
	resp, err := kapi.Set(context.Background(), resourceKey, resourceProv, nil)
	if err != nil {
		log.Fatal(err)
	} else {
		// print common key info
		log.Printf("Set is done. Metadata is %q\n", resp)
	}
	fmt.Printf("Getting value for %s\n", resourceKey)
	resp, err = kapi.Get(context.Background(), resourceKey, nil)
	if err != nil {
		log.Fatal(err)
	} else {
		// print common key info
		//log.Printf("Get is done. Metadata is %q\n", resp)
		// print value
		log.Printf("%q key has %q value\n", resp.Node.Key, resp.Node.Value)
	}
	//fmt.Println("Exiting storeProvenance")
}

func buildProvenance(parentResourceKind string, parentResourceName string, level int, 
	compositionTree *[]CompositionTreeNode) {
	//fmt.Printf("$$$$$ Building Provenance Level %d $$$$$ \n", level)
	childResourceKindList, present := compositionMap[parentResourceKind]
	if present {
		for _, childResourceKind := range childResourceKindList {
			childKindPlural := kindPluralMap[childResourceKind]
			childResourceApiVersion := kindVersionMap[childResourceKind]
			content := getResourceListContent(childResourceApiVersion, childKindPlural)
			metaDataAndOwnerReferenceList := parseMetaData(content)
			childrenList := filterChildren(&metaDataAndOwnerReferenceList, parentResourceName)
			compTreeNode := CompositionTreeNode{
				Level: level,
				ChildKind: childResourceKind,
				Children: childrenList,
			}
			*compositionTree = append(*compositionTree, compTreeNode)
			level = level + 1

			for _, metaDataRef := range childrenList {
				resourceName := metaDataRef.MetaDataName
				resourceKind := childResourceKind
				buildProvenance(resourceKind, resourceName, level, compositionTree)
			}
		}
	} else {
		return
	}
}

func getResourceListContent(resourceApiVersion, resourcePlural string) []byte {
	//fmt.Println("Entering getResourceListContent")
	url1 := fmt.Sprintf("https://%s:%s/%s/namespaces/%s/%s", serviceHost, servicePort, resourceApiVersion, namespace, resourcePlural)
	//fmt.Printf("Url:%s\n",url1)
	caToken := getToken()
	caCertPool := getCACert()
	u, err := url.Parse(url1)
	if err != nil {
	  panic(err)
	}
	req, err := http.NewRequest(httpMethod, u.String(), nil)
	if err != nil {
	    fmt.Println(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", string(caToken)))
	client := &http.Client{
	  Transport: &http.Transport{
	    TLSClientConfig: &tls.Config{
	        RootCAs: caCertPool,
	    },
	  },
	}
	resp, err := client.Do(req)
	if err != nil {
	    log.Printf("sending request failed: %s", err.Error())
	    fmt.Println(err)
	}
	defer resp.Body.Close()
	resp_body, _ := ioutil.ReadAll(resp.Body)

	//fmt.Println(resp.Status)
	//fmt.Println(string(resp_body))
	//fmt.Println("Exiting getResourceListContent")
	return resp_body
}

func parseMetaData(content []byte) []MetaDataAndOwnerReferences {
	//fmt.Println("Entering parseMetaData")
	var result map[string]interface{}
	json.Unmarshal([]byte(content), &result)
	// We need to parse following from the result
	// metadata.name
	// metadata.ownerReferences.name
	// metadata.ownerReferences.kind
	// metadata.ownerReferences.apiVersion
	//parentName := "podtest5-deployment"
	metaDataSlice := []MetaDataAndOwnerReferences{}
	items, ok := result["items"].([]interface{})

	if ok {
		for _, item := range items {
			//fmt.Println("=======================")
			itemConverted := item.(map[string]interface{})
			for key, value := range itemConverted {
				if key == "metadata" {
					//fmt.Println("----")
					//fmt.Println(key, value.(interface{}))
					metadataMap := value.(map[string]interface{})
					metaDataRef := MetaDataAndOwnerReferences{}
					for mkey, mvalue := range metadataMap {
						//fmt.Printf("%v ==> %v\n", mkey, mvalue.(interface{}))
						if mkey == "ownerReferences" {
							ownerReferencesList := mvalue.([]interface{})
							for _, ownerReference := range ownerReferencesList {
								ownerReferenceMap := ownerReference.(map[string]interface{})
								for okey, ovalue := range ownerReferenceMap {
									//fmt.Printf("%v --> %v\n", okey, ovalue)
									if okey == "name" {
										metaDataRef.OwnerReferenceName = ovalue.(string)
									}
									if okey == "kind" {
										metaDataRef.OwnerReferenceKind = ovalue.(string)
									}
									if okey == "apiVersion" {
										metaDataRef.OwnerReferenceAPIVersion = ovalue.(string)
									}
								}
							}
						}
						if mkey == "name" {
							metaDataRef.MetaDataName = mvalue.(string)
						}
					}
					metaDataSlice = append(metaDataSlice, metaDataRef)
				}
			}
		}
	}
	//fmt.Println("Exiting parseMetaData")
	return metaDataSlice
}

func filterChildren(metaDataSlice *[]MetaDataAndOwnerReferences, parentResourceName string) []MetaDataAndOwnerReferences {
	//fmt.Println("Entering filterChildren")
	metaDataSliceToReturn := []MetaDataAndOwnerReferences{}
	//fmt.Println("Printing the MetaDataSlice")
	for _, metaDataRef := range *metaDataSlice {
		if metaDataRef.OwnerReferenceName == parentResourceName {
			//fmt.Println("%v\n", metaDataRef)
			//fmt.Println("*************")
			metaDataSliceToReturn = append(metaDataSliceToReturn, metaDataRef)
		}
	}
	//fmt.Println("Exiting filterChildren")
	return metaDataSliceToReturn
}


func parse_prev(content []byte) map[string]string {
	var result map[string]interface{}
	json.Unmarshal([]byte(content), &result)

	// We need to parse following from the result
	// metadata.name
	// metadata.ownerReferences.name
	// metadata.ownerReferences.kind
	// metadata.ownerReferences.apiVersion

	var mapToReturn map[string]string

	items := result["items"].([]interface{})
	for _, item := range items {
		fmt.Println("=======================")
		itemConverted := item.(map[string]interface{})
		for key, value := range itemConverted {
			if key == "metadata" {
				fmt.Println("----")
				//fmt.Println(key, value.(interface{}))
				metadataMap := value.(map[string]interface{})
				for mkey, mvalue := range metadataMap {
					fmt.Printf("%v ==> %v\n", mkey, mvalue.(interface{}))
					if mkey == "ownerReferences" {
						ownerReferencesList := mvalue.([]interface{})
						for _, ownerReference := range ownerReferencesList {
							ownerReferenceMap := ownerReference.(map[string]interface{})
							for okey, ovalue := range ownerReferenceMap {
								fmt.Printf("%v --> %v\n", okey, ovalue)
							}
						}
					}
				}
				fmt.Println("----")
			}
		}
		fmt.Println("=======================")
	}
	fmt.Println("**************")
	fmt.Println("Map to Return:")
	for key, value := range mapToReturn {
		fmt.Printf("%v --> %v\n", key, value)
	}
	fmt.Println("**************")
	return mapToReturn
}


func getToken() []byte {
	caToken, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
	if err != nil {
	    panic(err) // cannot find token file
	}
	//fmt.Printf("Token:%s", caToken)
	return caToken
}

func getCACert() *cert.CertPool {
	caCertPool := cert.NewCertPool()
	caCert, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/ca.crt")
	if err != nil {
	    panic(err) // Can't find cert file
	}
	//fmt.Printf("CaCert:%s",caCert)
	caCertPool.AppendCertsFromPEM(caCert)
	return caCertPool
}


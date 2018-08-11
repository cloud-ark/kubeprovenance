package provenance

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"

	yaml "gopkg.in/yaml.v2"
	"k8s.io/apiserver/pkg/apis/audit/v1beta1"
)

var (
	serviceHost    string
	servicePort    string
	Namespace      string
	httpMethod     string
	etcdServiceURL string

	KindPluralMap  map[string]string
	kindVersionMap map[string]string
	compositionMap map[string][]string

	REPLICA_SET  string
	DEPLOYMENT   string
	POD          string
	CONFIG_MAP   string
	SERVICE      string
	ETCD_CLUSTER string

	AllProvenanceObjects []ProvenanceOfObject
)

type Event v1beta1.Event

//for example a postgres
type ObjectLineage map[int]Spec
type Spec struct {
	AttributeToData map[string]string
	Version         int
}

type ProvenanceOfObject struct {
	ObjectFullHistory ObjectLineage
	ResourcePlural    string
	Name              string
}

func init() {
	serviceHost = os.Getenv("KUBERNETES_SERVICE_HOST")
	servicePort = os.Getenv("KUBERNETES_SERVICE_PORT")
	Namespace = "default"
	httpMethod = http.MethodGet

	etcdServiceURL = "http://example-etcd-cluster-client:2379"

	DEPLOYMENT = "Deployment"
	REPLICA_SET = "ReplicaSet"
	POD = "Pod"
	CONFIG_MAP = "ConfigMap"
	SERVICE = "Service"
	ETCD_CLUSTER = "EtcdCluster"

	KindPluralMap = make(map[string]string)
	kindVersionMap = make(map[string]string)
	compositionMap = make(map[string][]string, 0)
	AllProvenanceObjects = make([]ProvenanceOfObject, 0)

}

func CollectProvenance() {
	// fmt.Println("Inside CollectProvenance")
	// for {

	//right now the way you set up the rest api depends on this compositionfile, so
	//postgreses type is not specified in it, so I do not think it would work for
	//crds like postgres right now
	readKindCompositionFile()
	parse()
	// 	time.Sleep(time.Second * 5)
	// }
}

func readKindCompositionFile() {
	// read from the opt file
	filePath := os.Getenv("KIND_COMPOSITION_FILE")
	yamlFile, err := ioutil.ReadFile(filePath)
	if err != nil {
		fmt.Printf("Error reading file:%s", err)
	}
	compositionsList := make([]composition, 0)
	err = yaml.Unmarshal(yamlFile, &compositionsList)
	for _, compositionObj := range compositionsList {
		kind := compositionObj.Kind
		endpoint := compositionObj.Endpoint
		composition := compositionObj.Composition
		plural := compositionObj.Plural
		KindPluralMap[kind] = plural
		kindVersionMap[kind] = endpoint
		compositionMap[kind] = composition
	}
}

func NewProvenanceOfObject() *ProvenanceOfObject {
	var s ProvenanceOfObject
	s.ObjectFullHistory = make(map[int]Spec) //need to generalize for other ObjectFullProvenances
	return &s
}

func NewSpec() *Spec {
	var s Spec
	s.AttributeToData = make(map[string]string)
	return &s
}

func FindProvenanceObjectByName(name string, allObjects []ProvenanceOfObject) *ProvenanceOfObject {
	for _, value := range allObjects {
		if name == value.Name {
			return &value
		}
	}
	return nil
}

func (s *Spec) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Version: %d\n", s.Version)
	for attribute, data := range s.AttributeToData {
		fmt.Fprintf(&b, "%s: %s\n", attribute, data)
	}
	return b.String()
}

func (o ObjectLineage) String() string {
	var b strings.Builder
	for version, spec := range o {
		fmt.Fprintf(&b, "Version: %d Data: %s\n", version, spec.String())
	}
	return b.String()
}

func (o ObjectLineage) LatestVersion() int {
	return len(o)
}

func (o ObjectLineage) GetVersions() string {
	arr := make([]string, 0)
	versions := o.LatestVersion()
	for index := 1; index <= versions; index++ {
		arr = append(arr, string(index))
	}
	return "[" + strings.Join(arr, ",") + "]"
}

//what happens if I delete the object?
//need to delete the ObjectFullProvenance for the object
//add type of ObjectFullProvenance, postgreses for example
func (o ObjectLineage) SpecHistory() string {
	s := make([]string, len(o))
	for v, spec := range o {
		s[v-1] = spec.String()
	}
	return strings.Join(s, "\n")
}
func (o ObjectLineage) Bisect(field, value string) string {
	s := make([]Spec, 0)
	for _, spec := range o {
		s = append(s, spec)
	}
	sort.Slice(s, func(i, j int) bool {
		return s[i].Version < s[i].Version
	})
	if len(s) == 1 {
		//check
		if s[0].AttributeToData[field] == value {
			return strconv.Itoa(1)
		}
	}
	for _, v := range s {
		if v.AttributeToData[field] == value {
			return strconv.Itoa(v.Version)
		}
	}
	return strconv.Itoa(-1)
}

func (o ObjectLineage) SpecHistoryInterval(vNumStart, vNumEnd int) []Spec {
	s := make([]Spec, vNumEnd-vNumStart+1)
	for v, spec := range o {
		if v >= vNumStart && v <= vNumEnd {
			s[v-vNumStart] = spec
		}
	}
	return s
}

func (o ObjectLineage) FullDiff(vNumStart, vNumEnd int) string {
	var b strings.Builder
	sp1 := o[vNumStart]
	sp2 := o[vNumEnd]
	for attribute, data1 := range sp1.AttributeToData {
		if data2, ok := sp2.AttributeToData[attribute]; ok {
			if data1 != data2 {
				fmt.Fprintf(&b, "FOUND DIFF")
				fmt.Fprintf(&b, "Spec version %d:\n %s\n", vNumStart, data1)
				fmt.Fprintf(&b, "Spec version %d:\n %s\n", vNumEnd, data2)
			} else {
				fmt.Fprintf(&b, "No difference for attribute %s \n", attribute)
			}
		} else { //for the case where a key exists in spec 1 that doesn't exist in spec 2
			fmt.Fprintf(&b, "FOUND DIFF")
			fmt.Fprintf(&b, "Spec version %d:\n %s\n", vNumStart, data1)
			fmt.Fprintf(&b, "Spec version %d:\n %s\n", vNumEnd, "No attribute found.")
		}
	}
	//for the case where a key exists in spec 2 that doesn't exist in spec 1
	for attribute, data1 := range sp2.AttributeToData {
		if _, ok := sp2.AttributeToData[attribute]; !ok {
			fmt.Fprintf(&b, "FOUND DIFF")
			fmt.Fprintf(&b, "Spec version %d:\n %s\n", vNumStart, "No attribute found.")
			fmt.Fprintf(&b, "Spec version %d:\n %s\n", vNumEnd, data1)
		}
	}
	return b.String()
}

func (o ObjectLineage) FieldDiff(fieldName string, vNumStart, vNumEnd int) string {
	var b strings.Builder
	data1, ok1 := o[vNumStart].AttributeToData[fieldName]
	data2, ok2 := o[vNumEnd].AttributeToData[fieldName]
	switch {
	case ok1 && ok2:
		if data1 != data2 {
			fmt.Fprintf(&b, "FOUND DIFF\n")
			fmt.Fprintf(&b, "Spec version %d:\n %s\n", vNumStart, data1)
			fmt.Fprintf(&b, "Spec version %d:\n %s\n", vNumEnd, data2)
		} else {
			fmt.Fprintf(&b, "No difference for attribute %s \n", fieldName)
		}
	case !ok1 && ok2:
		fmt.Fprintf(&b, "FOUND DIFF")
		fmt.Fprintf(&b, "Spec version %d:\n %s\n", vNumStart, "No attribute found.")
		fmt.Fprintf(&b, "Spec version %d:\n %s\n", vNumEnd, data2)
	case ok1 && !ok2:
		fmt.Fprintf(&b, "FOUND DIFF")
		fmt.Fprintf(&b, "Spec version %d:\n %s\n", vNumStart, data1)
		fmt.Fprintf(&b, "Spec version %d:\n %s\n", vNumEnd, "No attribute found.")
	case !ok1 && !ok2:
		fmt.Fprintf(&b, "Attribute not found in either version %d or %d", vNumStart, vNumEnd)
	}
	return b.String()
}

//Ref:https://www.sohamkamani.com/blog/2017/10/18/parsing-json-in-golang/#unstructured-data
func parse() {

	if _, err := os.Stat("/tmp/kube-apiserver-audit.log"); os.IsNotExist(err) {
		fmt.Println(fmt.Sprintf("could not stat the path %s", err))
		panic(err)
	}
	log, err := os.Open("/tmp/kube-apiserver-audit.log")
	if err != nil {
		fmt.Println(fmt.Sprintf("could not open the log file %s", err))
		panic(err)
	}
	defer log.Close()

	scanner := bufio.NewScanner(log)
	for scanner.Scan() {

		eventJson := scanner.Bytes()

		var event Event
		err := json.Unmarshal(eventJson, &event)
		if err != nil {
			s := fmt.Sprintf("Problem parsing event's json %s", err)
			fmt.Println(s)
		}

		var resourcePlural string
		var nameOfObject string
		// var namespace string

		//parse objectRef for unique object identifier and other fields
		resourcePlural = event.ObjectRef.Resource
		nameOfObject = event.ObjectRef.Name
		// namespace = event.ObjectReference.Namespace
		provObjPtr := FindProvenanceObjectByName(nameOfObject, AllProvenanceObjects)
		if provObjPtr == nil {
			//couldnt find object by name, make new provenance object bc This must be new
			provObjPtr = NewProvenanceOfObject()
			provObjPtr.ResourcePlural = resourcePlural
			provObjPtr.Name = nameOfObject
			AllProvenanceObjects = append(AllProvenanceObjects, *provObjPtr)
		}

		requestobj := event.RequestObject
		//now parse the spec into this provenanceObject that we found or created
		ParseRequestObject(provObjPtr, requestobj.Raw)
	}
	if err := scanner.Err(); err != nil {
		panic(err)
	}
	fmt.Println("Done parsing.")
}

func ParseRequestObject(objectProvenance *ProvenanceOfObject, requestObjBytes []byte) {
	fmt.Println("entering parse request")

	var result map[string]interface{}
	json.Unmarshal([]byte(requestObjBytes), &result)

	l1, ok := result["metadata"].(map[string]interface{})

	l2, ok := l1["annotations"].(map[string]interface{})
	if !ok {
		//sp, _ := result["spec"].(map[string]interface{})
		//TODO: for the case where a crd ObjectFullProvenance is first created, like initialize,
		//the metadata spec is empty. instead the spec field has the data
		//from the requestobjbytes:  metadata:map[creationTimestamp:<nil> name:client25 namespace:default]
		//instead of "requestObject":{
		//  "metadata":{
		//     "annotations":{
		//        "kubectl.kubernetes.io/last-applied-configuration":"{\"apiVersion\":\"postgrescontroller.kubeplus/v1\",\"kind\":\"Postgres\",\"metadata\":{\"annotations\":{},\"name\":\"client25\",\"namespace\":\"default\"},\"spec\":{\"databases\":[\"moodle\",\"wordpress\"],\"deploymentName\":\"client25\",\"image\":\"postgres:9.3\",\"replicas\":1,\"users\":[{\"password\":\"pass123\",\"username\":\"devdatta\"},{\"password\":\"pass123\",\"username\":\"pallavi\"}]}}\n"
		//     }
		//  },
		//fmt.Println("a: not ok") //hits here
		//fmt.Println(sp)
		return
	}
	l3, ok := l2["kubectl.kubernetes.io/last-applied-configuration"].(string)
	if !ok {
		// fmt.Println("b")
		// fmt.Println(l3)
		fmt.Println("Incorrect parsing of the auditEvent.requestObj.metadata")
	}
	in := []byte(l3)
	var raw map[string]interface{}
	json.Unmarshal(in, &raw)
	spec, ok := raw["spec"].(map[string]interface{})
	// fmt.Println("c")
	// fmt.Println(spec)
	if ok {
		fmt.Println("Successfully parsed")
	}
	newVersion := len(objectProvenance.ObjectFullHistory) + 1
	newSpec := buildSpec(spec)
	newSpec.Version = newVersion
	objectProvenance.ObjectFullHistory[newVersion] = newSpec
	fmt.Println("exiting parse request")
}
func buildSpec(spec map[string]interface{}) Spec {
	mySpec := *NewSpec()
	for attribute, value := range spec {
		bytes, err := json.MarshalIndent(value, "", "    ")
		if err != nil {
			fmt.Println("Error could not marshal json: " + err.Error())
		}
		attributeData := string(bytes)
		mySpec.AttributeToData[attribute] = attributeData
	}
	return mySpec
}

func printMaps() {
	fmt.Println("Printing kindVersionMap")
	for key, value := range kindVersionMap {
		fmt.Printf("%s, %s\n", key, value)
	}
	fmt.Println("Printing KindPluralMap")
	for key, value := range KindPluralMap {
		fmt.Printf("%s, %s\n", key, value)
	}
	fmt.Println("Printing compositionMap")
	for key, value := range compositionMap {
		fmt.Printf("%s, %s\n", key, value)
	}
}

func getResourceKinds() []string {
	resourceKindSlice := make([]string, 0)
	for key, _ := range compositionMap {
		resourceKindSlice = append(resourceKindSlice, key)
	}
	return resourceKindSlice
}

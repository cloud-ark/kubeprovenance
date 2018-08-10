package provenance

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
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

	ObjectFullProvenance Object
)

type Event v1beta1.Event

//for example a postgres
type Object map[int]Spec
type Spec struct {
	attributeToData map[string]string
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
}

func CollectProvenance() {
	// fmt.Println("Inside CollectProvenance")
	// for {
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

func NewSpec() *Spec {
	var s Spec
	s.attributeToData = make(map[string]string)
	return &s
}

func (s *Spec) String() string {
	var b strings.Builder
	for attribute, data := range s.attributeToData {
		fmt.Fprintf(&b, "Attribute: %s Data: %s\n", attribute, data)
	}
	return b.String()
}

func (o Object) String() string {
	var b strings.Builder
	for version, spec := range o {
		fmt.Fprintf(&b, "Version: %d Data: %s\n", version, spec.String())
	}
	return b.String()
}

func (o Object) LatestVersion(vNum int) int {
	return len(o)
}

func (o Object) Version(vNum int) Spec {
	return o[vNum]
}

//what happens if I delete the object?
//need to delete the ObjectFullProvenance for the object
//add type of ObjectFullProvenance, postgreses for example
func (o Object) SpecHistory() []string {
	s := make([]string, len(o))
	for v, spec := range o {
		s[v-1] = spec.String()
	}
	return s
}

//add type of ObjectFullProvenance, postgreses for example
func (o Object) SpecHistoryInterval(vNumStart, vNumEnd int) []Spec {
	s := make([]Spec, len(o))
	for v, spec := range o {
		if v >= vNumStart && v <= vNumEnd {
			s[v-1] = spec
		}
	}
	return s
}

//add type of ObjectFullProvenance, postgreses for example
func (o Object) FullDiff(vNumStart, vNumEnd int) string {
	var b strings.Builder
	sp1 := o[vNumStart]
	sp2 := o[vNumEnd]
	for attribute, data1 := range sp1.attributeToData {
		if data2, ok := sp2.attributeToData[attribute]; ok {
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
	for attribute, data1 := range sp2.attributeToData {
		if _, ok := sp2.attributeToData[attribute]; !ok {
			fmt.Fprintf(&b, "FOUND DIFF")
			fmt.Fprintf(&b, "Spec version %d:\n %s\n", vNumStart, "No attribute found.")
			fmt.Fprintf(&b, "Spec version %d:\n %s\n", vNumEnd, data1)
		}
	}
	return b.String()
}

//add type of ObjectFullProvenance, postgreses for example
func (o Object) FieldDiff(fieldName string, vNumStart, vNumEnd int) string {
	var b strings.Builder
	data1, ok1 := o[vNumStart].attributeToData[fieldName]
	data2, ok2 := o[vNumEnd].attributeToData[fieldName]
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

	if _, err := os.Stat("kube-apiserver-audit.log"); os.IsNotExist(err) {
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

		requestobj := event.RequestObject

		ParseRequestObject(requestobj.Raw)
	}
	if err := scanner.Err(); err != nil {
		panic(err)
	}
	fmt.Println("Done parsing.")
}

func ParseRequestObject(requestObjBytes []byte) {
	fmt.Println("entering parse request")

	var result map[string]interface{}
	json.Unmarshal([]byte(requestObjBytes), &result)

	l1, ok := result["metadata"].(map[string]interface{})
	if !ok {
		sp, _ := result["spec"].(map[string]interface{})
		//TODO: for the case where a crd ObjectFullProvenance is first created, like initialize,
		//the metadata spec is empty. instead the spec field has the data
		fmt.Println(sp)
		return
	}
	l2, ok := l1["annotations"].(map[string]interface{})
	l3, ok := l2["kubectl.kubernetes.io/last-applied-configuration"].(string)
	if !ok {
		fmt.Println("Incorrect parsing of the auditEvent.requestObj.metadata")
	}
	in := []byte(l3)
	var raw map[string]interface{}
	json.Unmarshal(in, &raw)
	spec, ok := raw["spec"].(map[string]interface{})

	if ok {
		fmt.Println("Successfully parsed")
	}

	saveProvenance(spec)

	fmt.Println("exiting parse request")
}
func saveProvenance(spec map[string]interface{}) {
	mySpec := *NewSpec()
	newVersion := 1 + len(ObjectFullProvenance)
	for attribute, value := range spec {
		bytes, err := json.MarshalIndent(value, "", "    ")
		if err != nil {
			fmt.Println("Error could not marshal json: " + err.Error())
		}
		attributeData := string(bytes)
		mySpec.attributeToData[attribute] = attributeData
	}
	ObjectFullProvenance[newVersion] = mySpec
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

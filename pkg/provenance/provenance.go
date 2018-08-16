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
	AttributeToData map[string]interface{}
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
	s.AttributeToData = make(map[string]interface{})
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

func (o ObjectLineage) GetVersions() string {
	s := make([]int, 0)
	for _, value := range o {
		s = append(s, value.Version)
	}
	sort.Ints(s)
	//get all versions, sort by version, make string array of them
	versions := make([]string, 0)
	for _, version := range s {
		versions = append(versions, fmt.Sprint(version)) //cast int to string
	}
	return "[" + strings.Join(versions, ", ") + "]\n"
}

//what happens if I delete the object?
//need to delete the ObjectFullProvenance for the object
//add type of ObjectFullProvenance, postgreses for example
func (o ObjectLineage) SpecHistory() string {
	s := make([]int, 0)
	for _, value := range o {
		s = append(s, value.Version)
	}
	sort.Ints(s)
	//get all versions, sort by version, make string array of them
	specs := make([]string, 0)
	for _, version := range s {
		specs = append(specs, fmt.Sprint(o[version])) //cast Spec to String
	}
	return strings.Join(specs, "\n")
}
func (o ObjectLineage) Bisect(field1, value1, field2, value2 string) string {
	s := make([]int, 0)
	for _, value := range o {
		s = append(s, value.Version)
	}
	sort.Ints(s)
	//get all versions, sort by version, make string array of them
	specs := make([]Spec, 0)
	for _, version := range s {
		specs = append(specs, o[version]) //cast Spec to String
	}
	// fmt.Println(specs)
	noSpecFound := fmt.Sprintf("Bisect for field %s: %s, %s: %s was not successful. Custom resource never reached this state.", field1, value1, field2, value2)

	if len(specs) == 1 {
		//check

		u, ok1 := specs[0].AttributeToData[field1]
		if !ok1 {
			fmt.Printf("Field %s not found.\n", field1)
			return noSpecFound
		}
		p, ok2 := specs[0].AttributeToData[field2]
		if !ok2 {
			fmt.Printf("Field %s not found.\n", field2)
			return noSpecFound
		}
		users, ok1 := p.([]string)
		if !ok1 {
			fmt.Printf("Type assertion failed. Underlying data is incorrect and is not a slice of strings: %s\n", u)
			return noSpecFound
		}
		passwords, ok2 := u.([]string)
		if !ok2 {
			fmt.Printf("Type assertion failed. Underlying data is incorrect and is not a slice of strings: %s\n", p)
			return noSpecFound
		}

		for i, v := range users {
			if value1 == v && passwords[i] == value2 {
				return "Version: " + strconv.Itoa(1)
			}
		}
	} else { //there is more than one spec. More than one Event found in the log
		for _, spec := range specs {
			//check
			u, ok1 := spec.AttributeToData[field1]
			if !ok1 {
				fmt.Printf("Field %s not found.\n", field1)
				return noSpecFound
			}
			p, ok2 := spec.AttributeToData[field2]
			if !ok2 {
				fmt.Printf("Field %s not found.\n", field2)
				return noSpecFound
			}
			users, ok1 := p.([]string)
			if !ok1 {
				fmt.Printf("Type assertion failed. Underlying data is incorrect and is not a slice of strings: %s\n", u)
				return noSpecFound
			}
			passwords, ok2 := u.([]string)
			if !ok2 {
				fmt.Printf("Type assertion failed. Underlying data is incorrect and is not a slice of strings: %s\n", p)
				return noSpecFound
			}

			for i1, v1 := range users {
				if value1 == v1 && passwords[i1] == value2 {
					return "Version: " + strconv.Itoa(spec.Version)
				}
			}
		}
	}

	return noSpecFound
}

//TODO: add optional parameters to spechistory route in apiserver.go, and call this method.
// Right now it is actually unused
func (o ObjectLineage) SpecHistoryInterval(vNumStart, vNumEnd int) string {
	//order keys so we append in order later, reference: https://blog.golang.org/go-maps-in-action#TOC_7.
	s := make([]int, 0)
	for _, value := range o {
		s = append(s, value.Version)
	}
	sort.Ints(s)
	//get all versions, sort by version, make string array of them
	specs := make([]Spec, 0)
	for _, version := range s {
		specs = append(specs, o[version]) //cast Spec to String
	}
	specStrings := make([]string, 0)
	for _, spec := range specs {
		if spec.Version >= vNumStart && spec.Version <= vNumEnd {
			specStrings = append(specStrings, spec.String())
		}
	}
	return strings.Join(specStrings, "\n")
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
	} else {
		fmt.Println("Unsuccessful parse")
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
		var isMap, isStringSlice, isString, isInt bool
		//note that I cannot do type assertions because the underlying data
		//of the interface{} is not a map[string]string or an []slice
		//so that means that every type assertion to
		//value.([]map[string]string) fails, neither will []string. have to cast, store that data as desired
		var mapSliceField []map[string]string
		bytes, _ := json.MarshalIndent(value, "", "    ")
		if err := json.Unmarshal(bytes, &mapSliceField); err == nil {
			isMap = true
		}
		var stringSliceField []string
		if err := json.Unmarshal(bytes, &stringSliceField); err == nil {
			isStringSlice = true
		}
		var plainStringField string
		if err := json.Unmarshal(bytes, &plainStringField); err == nil {
			isString = true
		}
		var intField int
		if err := json.Unmarshal(bytes, &intField); err == nil {
			isInt = true
		}
		switch {
		case isMap:
			//we don't know the keys and don't know the data
			//could be this for example:
			//usernames = [daniel, steve, jenny]
			//passwords = [22d732, 4343e2, 434343b]
			attributeToSlices := make(map[string][]string, 0)
			//build this and then i will loop through and add this to the spec
			for _, mapl := range mapSliceField { //this is an []map[string]string

				for key, data := range mapl {
					slice, ok := attributeToSlices[key]
					if ok {
						slice = append(slice, data)
						attributeToSlices[key] = slice
					} else { // first time seeing this key
						slice := make([]string, 0)
						slice = append(slice, data)
						attributeToSlices[key] = slice
					}
				}
			}

			//now add to the spec attributes
			for key, value := range attributeToSlices {
				mySpec.AttributeToData[key] = value
			}

		case isStringSlice:
			mySpec.AttributeToData[attribute] = stringSliceField
		case isString:
			mySpec.AttributeToData[attribute] = plainStringField
		case isInt:
			mySpec.AttributeToData[attribute] = intField
		default:
			fmt.Println(value)
			fmt.Println("Error with the spec data. not a map slice, float, int, string slice, or string.")
		}
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

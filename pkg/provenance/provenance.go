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
	Timestamp       string
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
	specs := make([]Spec, 0)
	for _, version := range s {
		specs = append(specs, o[version])
	}
	//get all versions, sort by version, make string array of them
	outputs := make([]string, 0)
	for _, spec := range specs {
		outputs = append(outputs, fmt.Sprintf("%s: Version %d", spec.Timestamp, spec.Version)) //cast int to string
	}
	return "[" + strings.Join(outputs, ", \n") + "]\n"
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
	specs := make([]Spec, 0)
	for _, version := range s {
		specs = append(specs, o[version]) //cast Spec to String
	}

	specStrings := make([]string, 0)
	for _, spec := range specs {
		specStrings = append(specStrings, spec.String())
	}
	return strings.Join(specStrings, "\n")
}

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
		specStrings = append(specStrings, spec.String())
	}
	return strings.Join(specStrings, "\n")
}

func (o ObjectLineage) Bisect(argMap map[string]string) string {
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
	allAttributeValuePairs := make([][]string, 0)
	for key, value := range argMap {
		attributeValueSlice := make([]string, 2)
		if strings.Contains(key, "field") {
			//find associated value in argMap
			fieldNum, err := strconv.Atoi(key[5:])
			if err != nil {
				return fmt.Sprintf("Failure, could not convert %s. Invalid Query parameters.", key)
			}
			//find assocaited value1
			valueOfKey, ok := argMap["value"+strconv.Itoa(fieldNum)]
			if !ok {
				return fmt.Sprintf("Could not find an associated value for field: %s", key)
			}
			attributeValueSlice[0] = value
			attributeValueSlice[1] = valueOfKey
			allAttributeValuePairs = append(allAttributeValuePairs, attributeValueSlice)
		}
	}
	// fmt.Printf("attributeValuePairs%s\n", allAttributeValuePairs)
	andGate := make([]bool, len(allAttributeValuePairs))
	for _, spec := range specs {
		index := 0
		satisfied := false
		for _, pair := range allAttributeValuePairs {
			qkey := pair[0]
			qval := pair[1]
			//each qkey qval has to be satisfied
			for mkey, mvalue := range spec.AttributeToData {
				vString, ok1 := mvalue.(string)
				if ok1 {
					fmt.Println("a")
					if qkey == mkey && qval == vString {
						satisfied = true
						break
					}
				}
				vStringSlice, ok2 := mvalue.([]string)
				if ok2 {
					fmt.Println("b")
					for _, str := range vStringSlice {
						if qkey == mkey && qval == str {
							satisfied = true
							break
						}
					}
				}
				vSliceMap, ok3 := mvalue.([]map[string]string)
				if ok3 {
					fmt.Println("c")
					for _, mymap := range vSliceMap {
						for okey, ovalue := range mymap {
							if qkey == okey && qval == ovalue {
								satisfied = true
								break
							}
						}
					}
				}

			}
			andGate[index] = satisfied
			index += 1
		}
		allTrue := true
		for _, b := range andGate {
			if !b {
				allTrue = false
			}
		}
		fmt.Println(andGate)
		if allTrue {
			return fmt.Sprintf("Version: %d", spec.Version)
		}
	}
	return "No version found that matches the query."
}

func (o ObjectLineage) FullDiff(vNumStart, vNumEnd int) string {
	var b strings.Builder
	sp1 := o[vNumStart]
	sp2 := o[vNumEnd]
	for attribute, data1 := range sp1.AttributeToData {
		data2, ok := sp2.AttributeToData[attribute] //check if the attribute even exists
		if ok {
			if data1 != data2 {
				fmt.Fprintf(&b, "Found diff on attribute %s:\n", attribute)
				fmt.Fprintf(&b, "\tVersion %d: %s\n", vNumStart, data1)
				fmt.Fprintf(&b, "\tVersion %d: %s\n", vNumEnd, data2)
			} else {
				// fmt.Fprintf(&b, "No difference for attribute %s \n", attribute)
			}

		} else { //for the case where a key exists in spec 1 that doesn't exist in spec 2
			fmt.Fprintf(&b, "Found diff on attribute %s:\n", attribute)
			fmt.Fprintf(&b, "\tVersion %d: %s\n", vNumStart, data1)
			fmt.Fprintf(&b, "\tVersion %d: %s\n", vNumEnd, "No attribute found.")
		}
	}
	//for the case where a key exists in spec 2 that doesn't exist in spec 1
	for attribute, data1 := range sp2.AttributeToData {
		if _, ok := sp2.AttributeToData[attribute]; !ok {
			fmt.Fprintf(&b, "Found diff on attribute %s:\n", attribute)
			fmt.Fprintf(&b, "\tVersion %d: %s\n", vNumStart, "No attribute found.")
			fmt.Fprintf(&b, "\tVersion %d: %s\n", vNumEnd, data1)
		}
	}
	return b.String()
}

func (o ObjectLineage) FieldDiff(fieldName string, vNumStart, vNumEnd int) string {
	var b strings.Builder
	data1, ok1 := o[vNumStart].AttributeToData[fieldName]
	data2, ok2 := o[vNumEnd].AttributeToData[fieldName]
	var stringSlice1, stringSlice2 string

	sliceStringData1, okTypeAssertion1 := data1.([]string)
	if okTypeAssertion1 {
		stringSlice1 = strings.Join(sliceStringData1, " ")
	}
	sliceStringData2, okTypeAssertion2 := data2.([]string) //type assertion to compare slices
	if okTypeAssertion2 {
		stringSlice2 = strings.Join(sliceStringData2, " ")
	}

	switch {
	case ok1 && ok2:
		if okTypeAssertion1 && okTypeAssertion2 {
			if stringSlice1 != stringSlice2 {
				fmt.Fprintf(&b, "Found diff on attribute %s:\n", fieldName)
				fmt.Fprintf(&b, "\tVersion %d: %s\n", vNumStart, stringSlice1)
				fmt.Fprintf(&b, "\tVersion %d: %s\n", vNumEnd, stringSlice2)
			} else {
				// fmt.Fprintf(&b, "No difference for attribute %s \n", attribute)
			}
		} else {
			if data1 != data2 {
				fmt.Fprintf(&b, "Found diff on attribute %s:\n", fieldName)
				fmt.Fprintf(&b, "\tSpec version %d: %s\n", vNumStart, data1)
				fmt.Fprintf(&b, "\tSpec version %d: %s\n", vNumEnd, data2)
			} else {
				// fmt.Fprintf(&b, "No difference for attribute %s \n", fieldName)
			}
		}
	case !ok1 && ok2:
		fmt.Fprintf(&b, "Found diff on attribute %s:\n", fieldName)
		fmt.Fprintf(&b, "\tVersion %d: %s\n", vNumStart, "No attribute found.")
		fmt.Fprintf(&b, "\tVersion %d: %s\n", vNumEnd, data2)
	case ok1 && !ok2:
		fmt.Fprintf(&b, "Found diff on attribute %s:\n", fieldName)
		fmt.Fprintf(&b, "\tVersion %d: %s\n", vNumStart, data1)
		fmt.Fprintf(&b, "\tVersion %d: %s\n", vNumEnd, "No attribute found.")
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
		timestamp := fmt.Sprint(event.RequestReceivedTimestamp.Format("2006-01-02 15:04:05"))
		//now parse the spec into this provenanceObject that we found or created
		ParseRequestObject(provObjPtr, requestobj.Raw, timestamp)
	}
	if err := scanner.Err(); err != nil {
		panic(err)
	}
	fmt.Println("Done parsing.")
}

func ParseRequestObject(objectProvenance *ProvenanceOfObject, requestObjBytes []byte, timestamp string) {
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
		fmt.Println("Unsuccessful parsed")
	}
	newVersion := len(objectProvenance.ObjectFullHistory) + 1
	newSpec := buildSpec(spec)
	newSpec.Version = newVersion
	newSpec.Timestamp = timestamp
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
			mySpec.AttributeToData[attribute] = mapSliceField
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

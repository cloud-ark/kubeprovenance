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
	"errors"
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
// Method to build a sorted slice of Spec from ObjectLineage map.
// Sorting is necessary because want to scan the spec versions in order, maps
// are unordered (ObjectLineage obj).
func getSpecsInOrder(o ObjectLineage) []Spec{
	// Get all versions, sort by version, make slice of specs
	s := make([]int, 0)
	for _, value := range o {
		s = append(s, value.Version)
	}
	sort.Ints(s)
	specs := make([]Spec, 0)
	for _, version := range s {
		specs = append(specs, o[version])
	}
	return specs
}
func (o ObjectLineage) GetVersions() string {
	specs := getSpecsInOrder(o)
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
	specs := getSpecsInOrder(o)
	specStrings := make([]string, 0)
	for _, spec := range specs {
		specStrings = append(specStrings, spec.String())
	}
	return strings.Join(specStrings, "\n")
}

func (o ObjectLineage) SpecHistoryInterval(vNumStart, vNumEnd int) string {
	specs := getSpecsInOrder(o)
	specStrings := make([]string, 0)
	for _, spec := range specs {
		specStrings = append(specStrings, spec.String())
	}
	return strings.Join(specStrings, "\n")
}
func buildAttributeRelationships(specs []Spec, allQueryPairs [][]string) map[string][][]string{
	// A map from top level attribute to array of pairs (represented as 2 len array)
	// mapRelationships with one top level object users, looks like this:
	// ex:	map[users:[[username pallavi] [password pass123]]]
	mapRelationships := make(map[string][][]string, 0)

	for _, spec := range specs {
		for _, pair := range allQueryPairs {
			for mkey, mvalue := range spec.AttributeToData {

				qkey := pair[0] //query key
				//each qkey qval has to be satisfied
				vSliceMap, ok := mvalue.([]map[string]string)
				if ok {
					for _, mymap := range vSliceMap {
						for okey, _ := range mymap {
							if qkey == okey {
								pairs, ok := mapRelationships[mkey]
								if ok {
									pairExistsAlready := false
									for _, v := range pairs{ //check existing pairs
											//pair[0] is the field, pair[1] is the value.
											if v[0] == pair[0]{
													pairExistsAlready = true
											}

									}
									//don't want duplicate fields, but want to catch all mapRelationships
									//over the lineage.
									if !pairExistsAlready{
										mapRelationships[mkey] = append(mapRelationships[mkey], pair)
									}
								} else {
									pairs = make([][]string, 0)
									pairs = append(pairs, pair)
									mapRelationships[mkey] = pairs
								}

							}
						}
					}
				}
			}
		}
	}
	return mapRelationships
}
func buildQueryPairsSlice(queryArgMap map[string]string) ([][]string, error){
	allQueryPairs := make([][]string, 0)
	//this section is storing the mappings from the query, into the allQueryPairs object
	//these are the queries, each element must be satisfied somewhere in the spec for the bisect to succeed
	for key, value := range queryArgMap {
		attributeValueSlice := make([]string, 2)
		if strings.Contains(key, "field") {
			//find associated value in argMap, the query params are such that field1=bar, field2=foo
			fieldNum, err := strconv.Atoi(key[5:])
			if err != nil {
				return nil,errors.New(fmt.Sprintf("Failure, could not convert %s. Invalid Query parameters.", key))

			}
			//find associated value by looking in the map for value+fieldNum.
			valueOfKey, ok := queryArgMap["value"+strconv.Itoa(fieldNum)]
			if !ok {
				return nil,errors.New(fmt.Sprintf("Could not find an associated value for field: %s", key))
			}
			attributeValueSlice[0] = value
			attributeValueSlice[1] = valueOfKey
			allQueryPairs = append(allQueryPairs, attributeValueSlice)
		}
	}
	return allQueryPairs,nil
}
//Steps taken in Bisect are:
//Sort the spec elements in order of their version number.

//Outer loop is going through each of the versions in order.
//First I parse the query into a slice of field/value pairs.
//Then build the map of related fields that belong to the same top level attribute
// and need to be found in the same underlying map. Then I looped over the
//field value pairs, searched based on the type.
func (o ObjectLineage) Bisect(argMap map[string]string) string {
	specs := getSpecsInOrder(o)

	allQueryPairs, err := buildQueryPairsSlice(argMap)
	if err!=nil{
		return err.Error()
	}
	// fmt.Printf("attributeValuePairs%s\n", allQueryPairs)

	// This method is to build the attributeRelationships from the Query
	// I Will use this to ensure that fields belonging to the same top-level
	// attribute, will be treated as a joint query. So you can't just
	// ask if username ever is daniel and password is ever 223843, because
	// it could find that in different parts of the spec. They both must be satisfied in the same map object
	mapRelationships := buildAttributeRelationships(specs, allQueryPairs)
	fmt.Printf("Query Attributes Map:%v\n", mapRelationships)
	// fmt.Println(specs)
	for _, spec := range specs {

		//every element represents whether a query pair was satisfied. they all must be true.
		//if they all are true, then that will be the version where the query is first satisfied.
		andGate := make([]bool, 0)
		for _, pair := range allQueryPairs {
			qkey := pair[0]
			qval := pair[1]
			satisfied := false
			for mkey, mvalue := range spec.AttributeToData {
				//search through the attributes in the spec. Possible types
				//are string, array of strings, and a map. so I will need to check these
				//with different search methods
				vString, ok := mvalue.(string)
				if ok { //if underlying value is a string
					satisfied = handleTrivialFields(vString, qkey, qval, mkey)
					if satisfied{
						break //qkey/qval was satisfied somewhere in the spec attributes, so move on to the next qkey/qval
					}
				}

				vStringSlice, ok := mvalue.([]string)
				if ok { //if it is a slice of strings
					satisfied = handleSimpleFields(vStringSlice, qkey, qval, mkey)
					if satisfied{
						break //qkey/qval was satisfied somewhere in the spec attributes, so move on to the next qkey/qval
					}
				}

				vSliceMap, ok := mvalue.([]map[string]string)
				//if it is a map, will need to check the underlying data to see
				//if the qkey/qval exists below the top level attribute. username exists
				//in the map contained by the 'users' attribute in the spec object, for example.
				if ok {
					satisfied = handleCompositeFields(vSliceMap, mapRelationships, qkey, qval, mkey)
					if satisfied{
						break //qkey/qval was satisfied somewhere in the spec attributes, so move on to the next qkey/qval
					}
				}
			}
			andGate = append(andGate, satisfied)
		}
		fmt.Printf("Result of checking all attributes:%v\n", andGate)
		allTrue := true
		for _, b := range andGate {
			if !b {
				allTrue = false
			}
		}
		//all indexes in andGate must be true
		if allTrue {
			return fmt.Sprintf("Version: %d", spec.Version)
		}
	}
	return "No version found that matches the query."
}
//this is for a field like deploymentName where the underyling state or data is a string
func handleTrivialFields(qkey, qval, mkey, fieldData string) bool{
	if qkey == mkey && qval == fieldData {
		return true
	}
	return false
}
//this is for a field like databases where the underyling state or data is a slice of strings.
func handleSimpleFields(vStringSlice []string, qkey, qval, mkey string) bool{
	satisfied := false
	for _, str := range vStringSlice {
		if qkey == mkey && qval == str {
			satisfied = true
			break
		}
	}
	return satisfied
}
func handleCompositeFields(vSliceMap []map[string]string, mapRelationships map[string][][]string, qkey, qval, mkey string) bool{
	for _, mymap := range vSliceMap { //looping through each elem in the mapSlice
		// check if there is any
		// other necessary requirements for this mkey.
		// if key exists, then multiple attributes have to be satisfied
		// at once for the query to work.

		//For example, say fields username and password belong to
		//an attribute in the spec called 'users'. The username and password
		//must be satisfied together in some element of vSliceMap since,
		//Since It cannot be the case where username is found in index 1 of vSliceMapSlice and password cannot
		//is found in index 2 of vSliceMapSlice.

		// find all the related attributes associated with the top-level specs
		// attribute mkey. this would be users for the postgres crd example.
		attributeCombinedQuery, ok := mapRelationships[mkey]

		if ok { //ensure multiple attributes are jointly satisfied
			jointQueryResults := make([]bool, 0)
			//jointQueryResults is a boolean slice that represents the satisfiability
			//of the joint query. (all need to be true for it to have found qkey to be true)
			for _, pair := range attributeCombinedQuery {
				qckey := pair[0] //for each field/value pair, must find each qckey in
												 //the map object mymap
				qcval := pair[1]
				pairSatisfied := false
				for okey, ovalue := range mymap {
					if qckey == okey && qcval == ovalue {
						pairSatisfied = true
					}
				}
				jointQueryResults = append(jointQueryResults, pairSatisfied)
			}

			allTrue := true
			for _, b := range jointQueryResults {
				if !b {
					allTrue = false
				}
			}
			// Note: I cannot just return allTrue because this breaks the outer loop ..
			// The logic goes: if allTrue is never set to True, out of all the elements in outer loop mymap,
			// then the loop will finish and return false. return allTrue with no if statement, would stop the
			// loop and only check one mymap! Need to scan through all before returning false
			if allTrue{//satisfied the joint query
				return allTrue
			}
		} else {
			//if there is no attribute relationship, but the mapslice type assert was fine,
			//only need to find an okey in one of the maps, where that query field/value (qkey,qvalue)
			//is satisfied.
			for okey, ovalue := range mymap {
				//If it never returns here, out of all the elements in mymap, Then
				//the qkey/qval was never found in the attributes within any map in the slice.
				if qkey == okey && qval == ovalue {
					return true
				}
			}
		}
	}
	return false
}
func (o ObjectLineage) FullDiff(vNumStart, vNumEnd int) string {
	var b strings.Builder
	sp1 := o[vNumStart]
	sp2 := o[vNumEnd]
	for attribute, data1 := range sp1.AttributeToData {
		data2, ok := sp2.AttributeToData[attribute] //check if the attribute even exists
		if ok {
			str1, ok1 := data1.(string)
			str2, ok2 := data2.(string)
			if ok1 && ok2 && str1 != str2 {
				fmt.Fprintf(&b, "Found diff on attribute %s:\n", attribute)
				fmt.Fprintf(&b, "\tVersion %d: %s\n", vNumStart, data1)
				fmt.Fprintf(&b, "\tVersion %d: %s\n", vNumEnd, data2)
			} else {
				// fmt.Fprintf(&b, "No difference for attribute %s \n", attribute)
			}
			int1, ok1 := data1.(int)
			int2, ok2 := data2.(int)
			if ok1 && ok2 && int1 != int2 {
				fmt.Fprintf(&b, "Found diff on attribute %s:\n", attribute)
				fmt.Fprintf(&b, "\tVersion %d: %d\n", vNumStart, int1)
				fmt.Fprintf(&b, "\tVersion %d: %d\n", vNumEnd, int2)
			} else {
				// fmt.Fprintf(&b, "No difference for attribute %s \n", attribute)
			}
			strArray1, ok1 := data1.([]string)
			strArray2, ok2 := data2.([]string)
			if ok1 && ok2 {
				for _, str := range strArray1 {
					found := false
					for _, val := range strArray2 {
						if str == val {
							found = true
						}
					}
					if !found { // if an element does not have a match in the next version
						fmt.Fprintf(&b, "Found diff on attribute %s:\n", attribute)
						fmt.Fprintf(&b, "\tVersion %d: %s\n", vNumStart, strArray1)
						fmt.Fprintf(&b, "\tVersion %d: %s\n", vNumEnd, strArray2)
					}
				}
			}
			strMap1, ok1 := data1.([]map[string]string)
			strMap2, ok2 := data2.([]map[string]string)
			if ok1 && ok2 {
				if len(strMap1) != len(strMap2) {
					fmt.Fprintf(&b, "Found diff on attribute %s:\n", attribute)
					fmt.Fprintf(&b, "\tVersion %d: %s\n", vNumStart, strMap1)
					fmt.Fprintf(&b, "\tVersion %d: %s\n", vNumEnd, strMap2)
				}
				if ToString(strMap1) != ToString(strMap2) {
					fmt.Fprintf(&b, "Found diff on attribute %s:\n", attribute)
					fmt.Fprintf(&b, "\tVersion %d: %s\n", vNumStart, strMap1)
					fmt.Fprintf(&b, "\tVersion %d: %s\n", vNumEnd, strMap2)
				}
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
func ToString(mapsp []map[string]string) string {
	var b strings.Builder
	for _, m := range mapsp {
		fmt.Fprintf(&b, "map{ ")
		for attribute, data := range m {
			fmt.Fprintf(&b, "%s: %s\n", attribute, data)
		}
		fmt.Fprintf(&b, " }\n")
	}
	return b.String()
}
func (o ObjectLineage) FieldDiff(fieldName string, vNumStart, vNumEnd int) string {
	var b strings.Builder
	data1, ok1 := o[vNumStart].AttributeToData[fieldName]
	data2, ok2 := o[vNumEnd].AttributeToData[fieldName]

	switch {
	case ok1 && ok2:
		str1, ok1 := data1.(string)
		str2, ok2 := data2.(string)
		if ok1 && ok2 && str1 != str2 {
			fmt.Fprintf(&b, "Found diff on attribute %s:\n", fieldName)
			fmt.Fprintf(&b, "\tVersion %d: %s\n", vNumStart, data1)
			fmt.Fprintf(&b, "\tVersion %d: %s\n", vNumEnd, data2)
		} else {
			// fmt.Fprintf(&b, "No difference for attribute %s \n", attribute)
		}
		int1, ok1 := data1.(int)
		int2, ok2 := data2.(int)
		if ok1 && ok2 && int1 != int2 {
			fmt.Fprintf(&b, "Found diff on attribute %s:\n", fieldName)
			fmt.Fprintf(&b, "\tVersion %d: %s\n", vNumStart, data1)
			fmt.Fprintf(&b, "\tVersion %d: %s\n", vNumEnd, data2)
		} else {
			// fmt.Fprintf(&b, "No difference for attribute %s \n", attribute)
		}
		strArray1, ok1 := data1.([]string)
		strArray2, ok2 := data2.([]string)
		if ok1 && ok2 {
			for _, str := range strArray1 {
				found := false
				for _, val := range strArray2 {
					if str == val {
						found = true
					}
				}
				if !found { // if an element does not have a match in the next version
					fmt.Fprintf(&b, "Found diff on attribute %s:\n", fieldName)
					fmt.Fprintf(&b, "\tVersion %d: %s\n", vNumStart, strArray1)
					fmt.Fprintf(&b, "\tVersion %d: %s\n", vNumEnd, strArray2)
				}
			}
		}
		strMap1, ok1 := data1.([]map[string]string)
		strMap2, ok2 := data2.([]map[string]string)
		if ok1 && ok2 {
			if len(strMap1) != len(strMap2) {
				fmt.Fprintf(&b, "Found diff on attribute %s:\n", fieldName)
				fmt.Fprintf(&b, "\tVersion %d: %s\n", vNumStart, strMap1)
				fmt.Fprintf(&b, "\tVersion %d: %s\n", vNumEnd, strMap2)
			}
			if ToString(strMap1) != ToString(strMap2) {
				fmt.Fprintf(&b, "Found diff on attribute %s:\n", fieldName)
				fmt.Fprintf(&b, "\tVersion %d: %s\n", vNumStart, strMap1)
				fmt.Fprintf(&b, "\tVersion %d: %s\n", vNumEnd, strMap2)
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
		fmt.Println("Incorrect parsing of the auditEvent.requestObj.metadata")
	}
	in := []byte(l3)
	var raw map[string]interface{}
	json.Unmarshal(in, &raw)
	spec, ok := raw["spec"].(map[string]interface{})
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
			// fmt.Println(value)
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

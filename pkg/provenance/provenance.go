package provenance

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

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

// Only used when I need to order the AttributeToData map for unit testing
type pair struct {
	Attribute string
	Data      interface{}
}
type OrderedMap []pair

//Similar to a map access ..
//returns Data, ok
func (o OrderedMap) At(attrib string) (interface{}, bool) {
	for _, my_pair := range o {
		if my_pair.Attribute == attrib {
			return my_pair.Data, true
		}
	}
	return nil, false
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

func onMinikube() bool {
	ip := os.Getenv("HOST_IP")
	fmt.Printf("Minikube IP:%s\n", ip)
	ipBeginsWith := strings.Split(ip, ".")[0]
	return ipBeginsWith == "10" || ipBeginsWith == "192"
	//status.hostIP is set to 10.0.2.15 for onMinikube
	//and 127.0.0.1 for the real kubernetes server
	//rc.yaml
	//- name: HOST_IP
	//	valueFrom:
	//		fieldRef:
	//			fieldPath: status.hostIP
}
func CollectProvenance() {
	fmt.Println("Inside CollectProvenance")
	readKindCompositionFile()
	useSample := false
	if onMinikube() {
		useSample = true
		parse(useSample) //using a sample audit log, because
		//currently audit logging is not supported for minikube
	} else {
		for { //keep looping because the audit-logging is live
			parse(useSample)
			time.Sleep(time.Second * 5)
		}
	}
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

// This String function must return the same output upon
// different calls. So I am doing ordering and sorting
// here because map is unordered and gives random outputs
// when printing
func (s *Spec) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Version: %d\n", s.Version)

	var keys []string
	for k, _ := range s.AttributeToData {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, attribute := range keys {
		data := s.AttributeToData[attribute]
		integer, ok := data.(int)
		if ok {
			fmt.Fprintf(&b, "  %s: %d\n", attribute, integer)
			continue
		}
		v, ok := data.([]map[string]string)
		if ok {
			fmt.Fprintf(&b, "  %s: [", attribute)
			for _, innermap := range v {
				fmt.Fprintf(&b, " map[")
				var innerkeys []string
				for k, _ := range innermap {
					innerkeys = append(innerkeys, k)
				}
				sort.Strings(innerkeys)
				var strs []string
				for _, k1 := range innerkeys {
					strs = append(strs, fmt.Sprintf("%s: %s", k1, innermap[k1]))
				}
				fmt.Fprintf(&b, strings.Join(strs, " "))
				fmt.Fprintf(&b, "] ")
			}
			fmt.Fprintf(&b, "]\n")
		} else {
			fmt.Fprintf(&b, "  %s: %s\n", attribute, data)
		}
	}

	return b.String()
}

// Returns the string representation of ObjectLineage
// used in GetHistory
func (o ObjectLineage) String() string {
	var b strings.Builder
	specs := getSpecsInOrder(o)

	for _, spec := range specs {
		fmt.Fprintf(&b, spec.String())
	}
	return b.String()
}

// Method to build a sorted slice of Spec from ObjectLineage map.
// Sorting is necessary because want to scan the spec versions in order, maps
// are unordered (ObjectLineage obj).
func getSpecsInOrder(o ObjectLineage) []Spec {
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
	return "[" + strings.Join(outputs, ",\n") + "]\n"
}

//https://stackoverflow.com/questions/23330781/sort-go-map-values-by-keys
func (o ObjectLineage) stringInterval(s, e int) string {
	var b strings.Builder
	specs := getSpecsInOrder(o)

	for _, spec := range specs {
		if spec.Version >= s && spec.Version <= e {
			fmt.Fprintf(&b, spec.String())
		}
	}
	return b.String()
}
func (o ObjectLineage) SpecHistory() string {
	return o.String()
}

func (o ObjectLineage) SpecHistoryInterval(vNumStart, vNumEnd int) string {
	if vNumStart < 0 {
		return "Invalid start parameter"
	}
	return o.stringInterval(vNumStart, vNumEnd)
}
func buildAttributeRelationships(specs []Spec, allQueryPairs [][]string) map[string][][]string {
	// Returns a map from top level attribute to array of pairs (represented as 2 len array)
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
									for _, v := range pairs { //check existing pairs
										//pair[0] is the field, pair[1] is the value.
										if v[0] == pair[0] {
											pairExistsAlready = true
										}

									}
									//don't want duplicate fields, but want to catch all mapRelationships
									//over the lineage.
									if !pairExistsAlready {
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
func buildQueryPairsSlice(queryArgMap map[string]string) ([][]string, error) {
	allQueryPairs := make([][]string, 0)
	//this section is storing the mappings from the query, into the allQueryPairs object
	//these are the queries, each element must be satisfied somewhere in the spec for the bisect to succeed
	for key, value := range queryArgMap {
		attributeValueSlice := make([]string, 2)
		if strings.Contains(key, "field") {
			//find associated value in argMap, the query params are such that field1=bar, field2=foo
			fieldNum, err := strconv.Atoi(key[5:])
			if err != nil {
				return nil, errors.New(fmt.Sprintf("Failure, could not convert %s. Invalid Query parameters.", key))

			}
			//find associated value by looking in the map for value+fieldNum.
			valueOfKey, ok := queryArgMap["value"+strconv.Itoa(fieldNum)]
			if !ok {
				return nil, errors.New(fmt.Sprintf("Could not find an associated value for field: %s", key))
			}
			attributeValueSlice[0] = value
			attributeValueSlice[1] = valueOfKey
			allQueryPairs = append(allQueryPairs, attributeValueSlice)
		}
	}
	return allQueryPairs, nil
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
	fmt.Printf("Query Attributes Map: %v\n", allQueryPairs)

	if err != nil {
		return err.Error()
	}
	// fmt.Printf("attributeValuePairs%s\n", allQueryPairs)

	// This method is to build the attributeRelationships from the Query
	// I Will use this to ensure that fields belonging to the same top-level
	// attribute, will be treated as a joint query. So you can't just
	// ask if username ever is daniel and password is ever 223843, because
	// it could find that in different parts of the spec. They both must be satisfied in the same map object
	mapRelationships := buildAttributeRelationships(specs, allQueryPairs)
	fmt.Printf("Query Attributes Same-parent-relationships: %v\n", mapRelationships)
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
					if satisfied {
						break //qkey/qval was satisfied somewhere in the spec attributes, so move on to the next qkey/qval
					}
				}

				vStringSlice, ok := mvalue.([]string)
				if ok { //if it is a slice of strings
					satisfied = handleSimpleFields(vStringSlice, qkey, qval, mkey)
					if satisfied {
						break //qkey/qval was satisfied somewhere in the spec attributes, so move on to the next qkey/qval
					}
				}

				vSliceMap, ok := mvalue.([]map[string]string)
				//if it is a map, will need to check the underlying data to see
				//if the qkey/qval exists below the top level attribute. username exists
				//in the map contained by the 'users' attribute in the spec object, for example.
				if ok {
					satisfied = handleCompositeFields(vSliceMap, mapRelationships, qkey, qval, mkey)
					if satisfied {
						break //qkey/qval was satisfied somewhere in the spec attributes, so move on to the next qkey/qval
					}
				}
			}
			andGate = append(andGate, satisfied)
		}
		// fmt.Printf("Result of checking all attributes: %v\n", andGate)
		allTrue := all(andGate)
		//all indexes in andGate must be true
		if allTrue {
			return fmt.Sprintf("Version: %d", spec.Version)
		}
	}
	return "No version found that matches the query."
}

//this is for a field like deploymentName where the underlying state or data is a string
func handleTrivialFields(qkey, qval, mkey, fieldData string) bool {
	if qkey == mkey && qval == fieldData {
		return true
	}
	return false
}

//this is for a field like databases where the underyling state or data is a slice of strings.
func handleSimpleFields(vStringSlice []string, qkey, qval, mkey string) bool {
	satisfied := false
	for _, str := range vStringSlice {
		if qkey == mkey && qval == str {
			satisfied = true
			break
		}
	}
	return satisfied
}
func handleCompositeFields(vSliceMap []map[string]string, mapRelationships map[string][][]string, qkey, qval, mkey string) bool {
	isPossible := false
	for _, m := range vSliceMap {
		for k, _ := range m {
			if k == qkey {
				isPossible = true
			}
		}
	}
	if !isPossible {
		//for example databases qkey will never pass a
		//composite field test since it is never a key under
		//the map vSliceMap
		// fmt.Printf("NOT POSSIBLE, %s %s\n", qkey, mkey)
		return false
	}

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

			allTrue := all(jointQueryResults)
			// Note: I cannot just return allTrue because this breaks the outer loop ..
			// The logic goes: if allTrue is never set to True, out of all the elements in outer loop mymap,
			// then the loop will finish and return false. return allTrue with no if statement, would stop the
			// loop and only check one mymap! Need to scan through all before returning false
			if allTrue { //satisfied the joint query
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

//Method that returns true if all the elements in boolSlice are True
func all(boolSlice []bool) bool {
	allTrue := true
	for _, b := range boolSlice {
		if !b {
			allTrue = false
		}
	}
	return allTrue
}

//Method that compares the elements within 2 mapSlices.
//Each map must have a corresponding map
func compareMaps(mapSlice1, mapSlice2 []map[string]string) bool {
	//little trick so that I loop through the bigger map slice,
	if len(mapSlice2) != len(mapSlice1) {
		return false
	}

	foundMatches := make([]bool, 0)
	for i := 0; i < len(mapSlice1); i++ {
		mapleft := mapSlice1[i]
		foundMatch := false
		//each element in map1, must have a elem in map2 that matches it.
		for j := 0; j < len(mapSlice2); j++ {
			mapright := mapSlice2[j]
			// Each attribute in left map must match an attribute in right map,
			// so andGate represents each attribute's match.
			andGate := make([]bool, 0)
			for lkey, lval := range mapleft {
				//if ok is true, that means that the keys matched.
				rval, ok := mapright[lkey]
				if ok && lval == rval {
					andGate = append(andGate, true)
				} else {
					andGate = append(andGate, false)
				}
			}

			foundMatch = all(andGate)
			// If foundMatch is true, then we found a match for mapleft, then break the map2 loop
			// and move on to the next element in mapLeft.
			if foundMatch {
				break
			}
		}
		foundMatches = append(foundMatches, foundMatch)
	}
	return all(foundMatches)
}

//Need some way to bring order to the elements of the AttributeToData map,
// because otherwise, the output is randomly ordered and I cannot unit test that.
// so This method orders the map based on the Attribute key and is similar
// to C++'s pair.
func (s Spec) OrderedPairs() OrderedMap {
	var keys []string
	for k, _ := range s.AttributeToData {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	orderedRet := make(OrderedMap, 0)
	for _, k1 := range keys {
		p := pair{}
		p.Attribute = k1
		p.Data = s.AttributeToData[k1]
		orderedRet = append(orderedRet, p)
	}
	return orderedRet
}
func (o ObjectLineage) FullDiff(vNumStart, vNumEnd int) string {
	var b strings.Builder
	sp1 := o[vNumStart].OrderedPairs()
	sp2 := o[vNumEnd].OrderedPairs()
	for _, my_pair := range sp1 {
		attr := my_pair.Attribute
		data1 := my_pair.Data
		data2, ok := sp2.At(attr) //check if the attribute even exists
		if ok {
			getDiff(&b, attr, data1, data2, vNumStart, vNumEnd)
		} else { //for the case where a key exists in spec 1 that doesn't exist in spec 2
			fmt.Fprintf(&b, "Found diff on attribute %s:\n", attr)
			fmt.Fprintf(&b, "  Version %d: %s\n", vNumStart, data1)
			fmt.Fprintf(&b, "  Version %d: %s\n", vNumEnd, "No attribute found.")
		}
	}
	//for the case where a key exists in spec 2 that doesn't exist in spec 1
	for _, my_pair := range sp2 {
		attr := my_pair.Attribute
		data1 := my_pair.Data
		if _, ok := sp2.At(attr); !ok {
			fmt.Fprintf(&b, "Found diff on attribute %s:\n", attr)
			fmt.Fprintf(&b, "  Version %d: %s\n", vNumStart, "No attribute found.")
			fmt.Fprintf(&b, "  Version %d: %s\n", vNumEnd, data1)
		}
	}
	return b.String()
}
func orderInnerMaps(m []map[string]string) []OrderedMap {
	orderedMaps := make([]OrderedMap, 0)
	for _, mi := range m {
		var keys []string
		for k, _ := range mi {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		orderedRet := make(OrderedMap, 0)
		for _, k1 := range keys {
			p := pair{}
			p.Attribute = k1
			p.Data = mi[k1]
			orderedRet = append(orderedRet, p)
		}
		orderedMaps = append(orderedMaps, orderedRet)
	}

	return orderedMaps
}
func getDiff(b *strings.Builder, fieldName string, data1, data2 interface{}, vNumStart, vNumEnd int) string {
	str1, ok1 := data1.(string)
	str2, ok2 := data2.(string)
	if ok1 && ok2 && str1 != str2 {
		fmt.Fprintf(b, "Found diff on attribute %s:\n", fieldName)
		fmt.Fprintf(b, "  Version %d: %s\n", vNumStart, data1)
		fmt.Fprintf(b, "  Version %d: %s\n", vNumEnd, data2)
	}
	int1, ok1 := data1.(int)
	int2, ok2 := data2.(int)
	if ok1 && ok2 && int1 != int2 {
		fmt.Fprintf(b, "Found diff on attribute %s:\n", fieldName)
		fmt.Fprintf(b, "  Version %d: %d\n", vNumStart, int1)
		fmt.Fprintf(b, "  Version %d: %d\n", vNumEnd, int2)
	}
	strArray1, ok1 := data1.([]string)
	strArray2, ok2 := data2.([]string)
	if ok1 && ok2 {
		sort.Strings(strArray1)
		sort.Strings(strArray2)

		//When the arrays are not the same len, found a difference.
		if len(strArray1) != len(strArray2) {
			fmt.Fprintf(b, "Found diff on attribute %s:\n", fieldName)
			fmt.Fprintf(b, "  Version %d: %s\n", vNumStart, strArray1)
			fmt.Fprintf(b, "  Version %d: %s\n", vNumEnd, strArray2)
		} else {
			for _, val1 := range strArray1 {
				found := false
				for _, val2 := range strArray2 {
					if val1 == val2 {
						found = true
					}
				}
				if !found { // if an element does not have a match in the next version
					fmt.Fprintf(b, "Found diff on attribute %s:\n", fieldName)
					fmt.Fprintf(b, "  Version %d: %s\n", vNumStart, strArray1)
					fmt.Fprintf(b, "  Version %d: %s\n", vNumEnd, strArray2)
					break
				}
			}
		}
	}
	strMap1, ok1 := data1.([]map[string]string)
	strMap2, ok2 := data2.([]map[string]string)
	if ok1 && ok2 {
		orderedInnerMap1 := orderInnerMaps(strMap1)
		orderedInnerMap2 := orderInnerMaps(strMap2)
		if !compareMaps(strMap1, strMap2) {
			fmt.Fprintf(b, "Found diff on attribute %s:\n", fieldName)
			fmt.Fprintf(b, "  Version %d: %s\n", vNumStart, orderedInnerMap1)
			fmt.Fprintf(b, "  Version %d: %s\n", vNumEnd, orderedInnerMap2)
		}
	}

	return b.String()
}
func (o ObjectLineage) FieldDiff(fieldName string, vNumStart, vNumEnd int) string {
	var b strings.Builder
	//Since this is a single field, do not have to do the OrderedMap business like the FullDiff.
	//Same outp everytime
	data1, ok1 := o[vNumStart].AttributeToData[fieldName]
	data2, ok2 := o[vNumEnd].AttributeToData[fieldName]
	switch {
	case ok1 && ok2:
		return getDiff(&b, fieldName, data1, data2, vNumStart, vNumEnd)
	case !ok1 && ok2:
		fmt.Fprintf(&b, "Found diff on attribute %s:\n", fieldName)
		fmt.Fprintf(&b, "  Version %d: %s\n", vNumStart, "No attribute found.")
		fmt.Fprintf(&b, "  Version %d: %s\n", vNumEnd, data2)
	case ok1 && !ok2:
		fmt.Fprintf(&b, "Found diff on attribute %s:\n", fieldName)
		fmt.Fprintf(&b, "  Version %d: %s\n", vNumStart, data1)
		fmt.Fprintf(&b, "  Version %d: %s\n", vNumEnd, "No attribute found.")
	case !ok1 && !ok2:
		fmt.Fprintf(&b, "Attribute not found in either version %d or %d", vNumStart, vNumEnd)
	}
	return b.String()
}

//Ref:https://www.sohamkamani.com/blog/2017/10/18/parsing-json-in-golang/#unstructured-data
func parse(useSample bool) {

	var log *os.File
	var err error
	if useSample {
		log, err = os.Open("/tmp/minikube-sample-audit.log")
		if err != nil {
			fmt.Println(fmt.Sprintf("could not open the log file %s", err))
			panic(err)
		}
	} else {
		if _, err := os.Stat("/tmp/kube-apiserver-audit.log"); os.IsNotExist(err) {
			fmt.Println(fmt.Sprintf("could not stat the path %s", err))
			panic(err)
		}
		log, err = os.Open("/tmp/kube-apiserver-audit.log")
		if err != nil {
			fmt.Println(fmt.Sprintf("could not open the log file %s", err))
			panic(err)
		}
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
		parseRequestObject(provObjPtr, requestobj.Raw, timestamp)
	}
	if err := scanner.Err(); err != nil {
		panic(err)
	}
}

//This method is to parse the bytes of the requestObject attribute of Event,
//build the spec object, and save that spec to the ObjectLineage map under the next version number.
func parseRequestObject(objectProvenance *ProvenanceOfObject, requestObjBytes []byte, timestamp string) {
	fmt.Println("entering parse request")
	var result map[string]interface{}
	json.Unmarshal([]byte(requestObjBytes), &result)

	map1, ok := result["metadata"].(map[string]interface{})

	map2, ok := map1["annotations"].(map[string]interface{})
	if !ok {
		//sp, _ := result["spec"].(map[string]interface{})
		//fmt.Println(sp)

		//TODO: for the case where a crd is first created, the
		//the annotations spec is empty, which is how subsequent requests are parsed.
		// instead all the data I want to parse is in the spec field.
		//Right now it is actually skipping this case.

		return
	}
	map3, ok := map2["kubectl.kubernetes.io/last-applied-configuration"].(string)
	if !ok {
		fmt.Println("Incorrect parsing of the auditEvent.requestObj.metadata")
	}
	in := []byte(map3)
	var raw map[string]interface{}
	json.Unmarshal(in, &raw)
	spec, ok := raw["spec"].(map[string]interface{})
	if ok {
		fmt.Println("Parse was successful!")
	} else {
		fmt.Println("Parse was unsuccessful!")
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

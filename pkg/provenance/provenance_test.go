package provenance

import (
	"fmt"
	"io/ioutil"
	"log"
	"testing"
	// "time"

	"github.com/jinzhu/copier"
	yaml "gopkg.in/yaml.v2"
)

//https://stackoverflow.com/questions/30947534/reading-a-yaml-file-in-golang
type conf struct {
	HistoryTest1         string `yaml:"HistoryTest1"`
	HistoryIntervalTest1 string `yaml:"HistoryIntervalTest1"`
	TestFullDiff1        string `yaml:"TestFullDiff1"`
	TestFullDiff2        string `yaml:"TestFullDiff2"`
	TestFullDiff3        string `yaml:"TestFullDiff3"`
	TestFieldDiff1       string `yaml:"TestFieldDiff1"`
	TestFieldDiff2       string `yaml:"TestFieldDiff2"`
	TestFieldDiff3       string `yaml:"TestFieldDiff3"`
	TestGetVersion1       string `yaml:"TestGetVersion1"`

}
// Parses an expected-output file 'conf' into some fields,
// which will be used during Unit tests
func (c *conf) getConf() *conf {

	yamlFile, err := ioutil.ReadFile("tests.yaml")
	if err != nil {
		log.Printf("yamlFile.Get err   #%v ", err)
	}
	err = yaml.Unmarshal(yamlFile, c)
	if err != nil {
		log.Fatalf("Unmarshal: %v", err)
	}
	return c
}

type Args struct {
	Version        int
	DeploymentName string
	Image          string
	Replicas       int
	Users          []map[string]string
	Databases      []string
}

func addUser(users []map[string]string, user, pass string) []map[string]string {

	s := make(map[string]string, 0)
	s["username"] = user
	s["password"] = pass
	users = append(users, s)
	return users
}
func removeUser(users []map[string]string, user, pass string) []map[string]string {
	copied := make([]map[string]string, len(users))
	copy(copied, users)
	for i, u := range copied {
		found := u["username"] == user
		if found {
			//This is how you delete from a slice in golang ...
			return append(copied[:i], copied[i+1:]...)
		}
	}
	//did not find the user in users.
	return copied
}
func updateUser(users []map[string]string, user, newPass string) []map[string]string {
	copied := make([]map[string]string, len(users))
	copy(copied, users)
	for _, u := range copied {
		found := u["username"] == user
		if found {
			u["password"] = newPass
		}
	}
	return copied
}
func addDatabase(databases []string, database string) []string {
	for _, d := range databases {
		//already exists
		if database == d {
			return databases
		}
	}

	databases = append(databases, database)
	return databases
}
func removeDatabase(databases []string, database string) []string {

	for i, d := range databases {
		if d == database {
			return append(databases[:i], databases[i+1:]...)
		}
	}
	//did not find the database to remove
	return databases
}

func makeSpec(a Args) Spec {
	attributeToData := make(map[string]interface{}, 0)
	attributeToData["deploymentName"] = a.DeploymentName
	attributeToData["image"] = a.Image
	attributeToData["replicas"] = a.Replicas
	attributeToData["users"] = a.Users
	attributeToData["databases"] = a.Databases
	s := Spec{}
	s.Version = a.Version
	// Using a filler date for unit testing
	s.Timestamp = "2006-01-02 15:04:05"
	s.AttributeToData = attributeToData
	return s
}
// Initializes an Args with some data
func initArgs() Args {
	ds := make([]string, 0)
	ds = append(ds, "servers")
	ds = append(ds, "staff")

	users := make([]map[string]string, 0)
	u1 := make(map[string]string, 0)
	u1["username"] = "daniel"
	u1["password"] = "4557832+#^"

	u2 := make(map[string]string, 0)
	u2["username"] = "kulkarni"
	u2["password"] = "5*7832!@$"

	users = append(users, u1)
	users = append(users, u2)
	a := Args{}
	a.Users = users
	a.Databases = ds
	a.Image = "postgres:9.3"
	a.Replicas = 3
	a.Version = 1
	a.DeploymentName = "MyDeployment"

	return a
}
// This method is meant to deep copy the Args datastructure.
// copier deep copies fields that are simple, but I had
// to deep copy maps and slices myself.
func deepCopyArgs(arg Args) Args {
	nArgs := Args{}
	err := copier.Copy(&nArgs, &arg)
	if err != nil {
		fmt.Printf("Error doing a deep copy of Args: %s", err)
		return nArgs
	}

	//deep copying for maps,
	// tried two popular libraries and they didn't work..
	newUsers := make([]map[string]string, 0)
	for _, m := range nArgs.Users {
		u := make(map[string]string, 0)
		for k, v := range m {
			u[k] = v
		}
		newUsers = append(newUsers, u)

	}
	newDb := make([]string, 0)
	for _, m := range nArgs.Databases {
		newDb = append(newDb, m)
	}
	//The above code successfully made a deep copy of the map
	nArgs.Users = newUsers
	nArgs.Databases = newDb
	return nArgs
}
//This method builds some lineage data
func buildLineage() (ObjectLineage, Args) {
	objectLineage := make(map[int]Spec)
	args := initArgs()
	spec1 := makeSpec(args)
	objectLineage[spec1.Version] = spec1

	args = deepCopyArgs(args)
	users2 := addUser(args.Users, "johnson", "fluffy23")
	args.Users = users2
	args.Version = 2
	spec2 := makeSpec(args)
	objectLineage[spec2.Version] = spec2

	args = deepCopyArgs(args)
	users3 := addUser(args.Users, "thomas", "bedsheet85")
	args.Users = users3
	args.Version = 3
	spec3 := makeSpec(args)
	objectLineage[spec3.Version] = spec3

	args = deepCopyArgs(args)
	db1 := addDatabase(args.Databases, "logging")
	args.Databases = db1
	args.Version = 4
	spec4 := makeSpec(args)
	objectLineage[spec4.Version] = spec4

	args = deepCopyArgs(args)
	db2 := addDatabase(args.Databases, "demographics")
	args.Databases = db2
	args.Version = 5
	spec5 := makeSpec(args)
	objectLineage[spec5.Version] = spec5
	args = deepCopyArgs(args)
	return objectLineage, args
}

// This method is testing that the joint functionality of Bisect works
// with a complex query involving a complex query of 3 fields
func TestBisectGeneral(t *testing.T) {
	objLineage, _ := buildLineage()
	argMapTest := make(map[string]string, 0)
	argMapTest["field1"] = "databases"
	argMapTest["value1"] = "demographics"
	argMapTest["field2"] = "username"
	argMapTest["value2"] = "daniel"
	argMapTest["field3"] = "password"
	argMapTest["value3"] = "4557832+#^"

	vOutput := objLineage.Bisect(argMapTest)
	expected := "Version: 5"
	if vOutput != expected {
		t.Errorf("Version output for TestBisectGeneral() was incorrect, got: %s, want: %s.\n", vOutput, expected)
	}
}
// This method changes a user's password, and is testing
// whether the bisect will take a query with the same
// parent element (user), and process a complicated
// query involving maps.
func TestBisectPasswordChanged(t *testing.T) {
	objLineage, newArgs := buildLineage()

	users := updateUser(newArgs.Users, "daniel", "JEK873BUL!")
	db := removeDatabase(newArgs.Databases, "logging")
	newArgs.Users = users
	newArgs.Databases = db
	newArgs.Version = 6
	spec6 := makeSpec(newArgs)
	objLineage[spec6.Version] = spec6
	argMapTest := make(map[string]string, 0)
	argMapTest["field1"] = "username"
	argMapTest["value1"] = "daniel"
	argMapTest["field2"] = "password"
	argMapTest["value2"] = "JEK873BUL!"

	vOutput := objLineage.Bisect(argMapTest)
	expected := "Version: 6"
	if vOutput != expected {
		t.Errorf("Version output for TestBisectPasswordChanged() was incorrect, got: %s, want: %s.\n", vOutput, expected)
	}
}
// This method removes a databases, and
// is testing that Bisect still works.
func TestBisectRemoveDatabase(t *testing.T) {
	objLineage, newArgs := buildLineage()
	args := deepCopyArgs(newArgs)

	db := removeDatabase(args.Databases, "logging")
	args.Databases = db
	args.Version = 6
	spec6 := makeSpec(args)
	objLineage[spec6.Version] = spec6

	argMapTest := make(map[string]string, 0)
	argMapTest["field1"] = "databases"
	argMapTest["value1"] = "logging"

	vOutput := objLineage.Bisect(argMapTest)
	expected := "Version: 4"
	if vOutput != expected {
		t.Errorf("Version output for TestBisectRemoveDatabase() was incorrect, got: %s, want: %s.\n", vOutput, expected)
	}
}

// This method adds a new databases, admins, and
// is testing Bisect on a slice type.
func TestBisectAddDatabase(t *testing.T) {
	objLineage, newArgs := buildLineage()

	db := addDatabase(newArgs.Databases, "admins")
	newArgs.Databases = db
	newArgs.Version = 6
	spec6 := makeSpec(newArgs)
	objLineage[spec6.Version] = spec6

	argMapTest := make(map[string]string, 0)
	argMapTest["field1"] = "databases"
	argMapTest["value1"] = "admins"

	vOutput := objLineage.Bisect(argMapTest)
	expected := "Version: 6"
	if vOutput != expected {
		t.Errorf("Version output for TestBisectAddDatabase() was incorrect, got: %s, want: %s.", vOutput, expected)
	}
}
// This method removes a user, and checks if Bisect works as intended
func TestBisectRemoveUser(t *testing.T) {
	objLineage, newArgs := buildLineage()
	users := removeUser(newArgs.Users, "daniel", "JEK873BUL!")
	newArgs.Users = users
	newArgs.Version = 6
	spec6 := makeSpec(newArgs)
	objLineage[spec6.Version] = spec6

	argMapTest := make(map[string]string, 0)
	argMapTest["field1"] = "username"
	argMapTest["value1"] = "daniel"
	argMapTest["field2"] = "password"
	argMapTest["value2"] = "4557832+#^"

	vOutput := objLineage.Bisect(argMapTest)
	expected := "Version: 1"
	if vOutput != expected {
		t.Errorf("Version output for Bisect() was incorrect, got: %s, want: %s.\n", vOutput, expected)
	}
}
// This method adds a new user, Stevejobs, and
// ensures that a bisect query will see this.
func TestBisectAddUser(t *testing.T) {
	objLineage, newArgs := buildLineage()
	users := addUser(newArgs.Users, "Stevejobs", "apple468")
	newArgs.Users = users
	newArgs.Version = 6
	spec6 := makeSpec(newArgs)
	objLineage[spec6.Version] = spec6

	argMapTest := make(map[string]string, 0)
	argMapTest["field1"] = "username"
	argMapTest["value1"] = "Stevejobs"
	argMapTest["field2"] = "password"
	argMapTest["value2"] = "apple468"

	vOutput := objLineage.Bisect(argMapTest)
	expected := "Version: 6"
	if vOutput != expected {
		t.Errorf("Version output for TestBisectAddUser() was incorrect, got: %s, want: %s.", vOutput, expected)
	}
}
// This Unit test changes a string field and ensures that the Bisect
// will pick up this change.
func TestBisectChangeDeploymentName(t *testing.T) {
	objLineage, newArgs := buildLineage()
	newArgs.DeploymentName = "Deployment66"
	newArgs.Version = 6
	spec6 := makeSpec(newArgs)
	objLineage[spec6.Version] = spec6

	argMapTest := make(map[string]string, 0)
	argMapTest["field1"] = "deploymentName"
	argMapTest["value1"] = "Deployment66"

	vOutput := objLineage.Bisect(argMapTest)
	expected := "Version: 6"
	if vOutput != expected {
		t.Errorf("Version output for TestBisectChangeDeploymentName() was incorrect, got: %s, want: %s.\n", vOutput, expected)
	}
}
func TestHistory(t *testing.T) {
	objLineage, _ := buildLineage()

	vOutput := objLineage.SpecHistory()
	var c conf
	c.getConf()
	expected := c.HistoryTest1

	if vOutput != expected {
		t.Errorf("History output for TestHistory() was incorrect, got: %s, want: %s.\n", vOutput, expected)
	}
}

func TestHistoryInterval(t *testing.T) {
	objLineage, _ := buildLineage()

	vOutput := objLineage.SpecHistoryInterval(2, 4)
	var c conf
	c.getConf()
	expected := c.HistoryIntervalTest1

	if vOutput != expected {
		t.Errorf("History output for TestHistoryInterval() was incorrect, got: %s, want: %s.\n", vOutput, expected)
	}
}

//Tests changes made to ordinary string literals
func TestFieldDiff1(t *testing.T) {
	objLineage, newArgs := buildLineage()
	newArgs.DeploymentName = "Deployment66"
	newArgs.Version = 6
	spec6 := makeSpec(newArgs)
	objLineage[spec6.Version] = spec6
	output := objLineage.FieldDiff("deploymentName", 5, 6)
	var c conf
	c.getConf()
	expected := c.TestFieldDiff1

	if output != expected {
		t.Errorf("Diff output for TestFieldDiff1() was incorrect, got: %s, want: %s.\n", output, expected)
	}
}

// Tests the edge case where an element in a slice has changed and the slices are
// the same length.
func TestFieldDiff2(t *testing.T) {
	objLineage, newArgs := buildLineage()
	databasest := removeDatabase(newArgs.Databases, "demographics")
	databases := addDatabase(databasest, "testing")

	newArgs.Databases = databases
	newArgs.Version = 6
	spec6 := makeSpec(newArgs)
	objLineage[spec6.Version] = spec6
	output := objLineage.FieldDiff("databases", 5, 6)

	var c conf
	c.getConf()
	expected := c.TestFieldDiff2
	if output != expected {
		t.Errorf("Diff output for TestFieldDiff2() was incorrect, got: %s, want: %s.\n", output, expected)
	}
}

//Testing diff changes for an ordinary int alteration
func TestFieldDiff3(t *testing.T) {
	objLineage, newArgs := buildLineage()

	newArgs.Replicas = 10
	newArgs.Version = 6
	spec6 := makeSpec(newArgs)
	objLineage[spec6.Version] = spec6
	output := objLineage.FieldDiff("replicas", 5, 6)

	var c conf
	c.getConf()
	expected := c.TestFieldDiff3
	if output != expected {
		t.Errorf("Diff output for TestFieldDiff3() was incorrect, got: %s, want: %s.\n", output, expected)
	}
}
// Tests the Diff output for altering a string field
func TestFullDiff1(t *testing.T) {
	objLineage, newArgs := buildLineage()
	newArgs.DeploymentName = "Deployment66"
	newArgs.Version = 6
	spec6 := makeSpec(newArgs)
	objLineage[spec6.Version] = spec6

	output := objLineage.FullDiff(5, 6)
	var c conf
	c.getConf()
	expected := c.TestFullDiff1

	if output != expected {
		t.Errorf("Diff output for TestFullDiff1() was incorrect, got: %s, want: %s.\n", output, expected)
	}
}
// Tests Diff functionality for when a user is added.
func TestFullDiff2(t *testing.T) {
	objLineage, newArgs := buildLineage()
	users := addUser(newArgs.Users, "Stevejobs", "apple468")
	newArgs.Users = users
	newArgs.Version = 6
	spec6 := makeSpec(newArgs)
	objLineage[spec6.Version] = spec6

	output := objLineage.FullDiff(5, 6)
	var c conf
	c.getConf()
	expected := c.TestFullDiff2

	if output != expected {
		t.Errorf("Diff output for TestFullDiff2() was incorrect, got: %s, want: %s.\n", output, expected)
	}
}

// This method tests FullDiff, by making a combination of changes on a custom resource
// In doing so, tests the edge case where slices (such as Databases) is not equal
func TestFullDiff3(t *testing.T) {
	objLineage, newArgs := buildLineage()
	users := addUser(newArgs.Users, "Stevejobs", "apple468")
	databases := addDatabase(newArgs.Databases, "testing")

	newArgs.Users = users
	newArgs.Databases = databases
	newArgs.Version = 6
	spec6 := makeSpec(newArgs)
	objLineage[spec6.Version] = spec6
	output := objLineage.FullDiff(5, 6)

	var c conf
	c.getConf()
	expected := c.TestFullDiff3

	if output != expected {
		t.Errorf("Diff output for TestFullDiff3() was incorrect, got: %s, want: %s.\n", output, expected)
	}
}
// Tests GetVersions for when a new version is added
func TestGetVersion1(t *testing.T) {
	objLineage, newArgs := buildLineage()
	databases := addDatabase(newArgs.Databases, "testing")

	newArgs.Databases = databases
	newArgs.Version = 6
	spec6 := makeSpec(newArgs)
	objLineage[spec6.Version] = spec6
	output := objLineage.GetVersions()
	var c conf
	c.getConf()
	expected := c.TestGetVersion1

	if output != expected {
		t.Errorf("Versions output for TestFullDiff3() was incorrect, got: %s, want: %s.\n", output, expected)
	}
}

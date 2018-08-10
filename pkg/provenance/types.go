package provenance

// Used for unmarshalling JSON output from the main API server
type composition struct {
	Kind        string   `yaml:"kind"`
	Plural      string   `yaml:"plural"`
	Endpoint    string   `yaml:"endpoint"`
	Composition []string `yaml:"composition"`
}

// Used for Final output
type Composition struct {
	Level    int
	Kind     string
	Name     string
	Status   string
	Children []Composition
}

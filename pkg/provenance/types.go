package provenance

import (
	"sync"
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

type ClusterProvenance struct {
	clusterProvenance []Provenance
	mux sync.Mutex
}

var (
	TotalClusterProvenance ClusterProvenance
)

func init() {
	TotalClusterProvenance = ClusterProvenance{}
}
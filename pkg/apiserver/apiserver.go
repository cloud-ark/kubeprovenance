package apiserver

import (
	"fmt"
	"os"
	"io/ioutil"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"github.com/emicklei/go-restful"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/cloud-ark/kubeprovenance/pkg/provenance"
)

const GroupName = "kubeprovenance.cloudark.io"
const GroupVersion = "v1"

var (
	Scheme = runtime.NewScheme()
	Codecs = serializer.NewCodecFactory(Scheme)
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	AddToScheme   = SchemeBuilder.AddToScheme
    SchemeGroupVersion = schema.GroupVersion{Group: GroupName, Version: GroupVersion}
)

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion)
	return nil
}

func init() {
	utilruntime.Must(AddToScheme(Scheme))
	utilruntime.Must(Scheme.SetVersionPriority(SchemeGroupVersion))
	// we need to add the options to empty v1
	// TODO fix the server code to avoid this
	metav1.AddToGroupVersion(Scheme, schema.GroupVersion{Version: GroupVersion})

	// TODO: keep the generic API server from wanting this
	unversioned := schema.GroupVersion{Group: "", Version: GroupVersion}
	Scheme.AddUnversionedTypes(unversioned,
		&metav1.Status{},
		&metav1.APIVersions{},
		&metav1.APIGroupList{},
		&metav1.APIGroup{},
		&metav1.APIResourceList{},
	)

	// Start collecting provenance
	go provenance.CollectProvenance()
}

type ExtraConfig struct {
	// Place you custom config here.
}

type Config struct {
	GenericConfig *genericapiserver.RecommendedConfig
	ExtraConfig   ExtraConfig
}

// ProvenanceServer contains state for a Kubernetes cluster master/api server.
type ProvenanceServer struct {
	GenericAPIServer *genericapiserver.GenericAPIServer
}

type completedConfig struct {
	GenericConfig genericapiserver.CompletedConfig
	ExtraConfig   *ExtraConfig
}

type CompletedConfig struct {
	// Embed a private pointer that cannot be instantiated outside of this package.
	*completedConfig
}

// Complete fills in any fields not set that are required to have valid data. It's mutating the receiver.
func (cfg *Config) Complete() CompletedConfig {
	c := completedConfig{
		cfg.GenericConfig.Complete(),
		&cfg.ExtraConfig,
	}

	c.GenericConfig.Version = &version.Info{
		Major: "1",
		Minor: "0",
	}

	return CompletedConfig{&c}
}

// New returns a new instance of ProvenanceServer from the given config.
func (c completedConfig) New() (*ProvenanceServer, error) {
	genericServer, err := c.GenericConfig.New("kube provenance server", genericapiserver.NewEmptyDelegate())
	if err != nil {
		return nil, err
	}

	s := &ProvenanceServer{
		GenericAPIServer: genericServer,
	}

	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(GroupName, Scheme, metav1.ParameterCodec, Codecs)

	if err := s.GenericAPIServer.InstallAPIGroup(&apiGroupInfo); err != nil {
		return nil, err
	}

	//installProvenanceWebService_in_cluster(s)

	installProvenanceWebService(s)

	return s, nil
}

func installProvenanceWebService(provenanceServer *ProvenanceServer) {
	fmt.Printf("========== 8 ===============\n")
	resourceKindList := []string{"Deployments","EtcdClusters"}
	for _, resourceKind := range resourceKindList {
		path := "/apis/" + GroupName + "/" + GroupVersion + "/compositions/" + resourceKind
		fmt.Println("WS PATH:" + path)
		ws := getProvenanceWebService()
		ws.Path(path).
			Consumes(restful.MIME_JSON, restful.MIME_XML).
			Produces(restful.MIME_JSON, restful.MIME_XML)
		ws.Route(ws.GET("/{resource-id}").To(getCompositions))
		provenanceServer.GenericAPIServer.Handler.GoRestfulContainer.Add(ws)

		path1 := "/apis/" + GroupName + "/" + GroupVersion + "/namespaces/default/" + strings.ToLower(resourceKind)
		fmt.Println("WS PATH1:" + path1)
		ws1 := getProvenanceWebService()
		ws1.Path(path1).
			Consumes(restful.MIME_JSON, restful.MIME_XML).
			Produces(restful.MIME_JSON, restful.MIME_XML)
		ws1.Route(ws1.GET("/{resource-id}/compositions").To(getCompositions))
		provenanceServer.GenericAPIServer.Handler.GoRestfulContainer.Add(ws1)
	}
}

func installProvenanceWebService_in_cluster(provenanceServer *ProvenanceServer) {
	fmt.Printf("========== 7 ===============\n")

	// Read file location from environment variable
     filePath := os.Getenv("KUBEPLUS_PLATFORM_FILE")
     content, err := ioutil.ReadFile(filePath)
     if err != nil {
     	fmt.Printf("Error reading file:%s", err)
     }

     lines := strings.Split(string(content), "\n")

     for _, crd := range lines {
     	if crd != "" {
     		fmt.Printf("CRD:%v\n", crd)
			fmt.Printf("========== 8 ===============\n")
		    //path := "/apis/" + GroupName + "/v1alpha1/namespaces/default/" + crd

			path := "/apis/" + GroupName + "/compositions/"

			ws := getProvenanceWebService()
			ws.Path(path).
				Consumes(restful.MIME_JSON, restful.MIME_XML).
				Produces(restful.MIME_JSON, restful.MIME_XML)
			ws.Route(ws.GET("/" + crd + "/{crd-id}/").To(getCompositions))
			provenanceServer.GenericAPIServer.Handler.GoRestfulContainer.Add(ws)
		}
	}
}

func getProvenanceWebService() *restful.WebService {
	ws := new(restful.WebService)
	ws.Path("/apis")
	// a.prefix contains "prefix/group/version"
	ws.Consumes("*/*")
	//mediaTypes, streamMediaTypes := negotiation.MediaTypesForSerializer(a.group.Serializer)
	//ws.Produces(append(mediaTypes, streamMediaTypes...)...)
	ws.Produces(restful.MIME_JSON, restful.MIME_XML)
	ws.ApiVersion(GroupName)
	return ws
}

func getCompositions(request *restful.Request, response *restful.Response) {
	fmt.Printf("========== AAAAA ===============\n")
	resName := request.PathParameter("resource-id")
	stringToReturn := "Hello there - THis is compositon of Resource name:%s" + resName + " -- Pod1 Pod2 Service2\n"

	fmt.Printf("Printing Provenance\n") 
	provenance.TotalClusterProvenance.PrintProvenance()

	response.WriteEntity(stringToReturn)
}


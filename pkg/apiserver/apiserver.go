package apiserver

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/emicklei/go-restful"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/version"
	genericapiserver "k8s.io/apiserver/pkg/server"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/cloud-ark/kubeprovenance/pkg/provenance"
)

const GroupName = "kubeprovenance.cloudark.io"
const GroupVersion = "v1"

var (
	Scheme             = runtime.NewScheme()
	Codecs             = serializer.NewCodecFactory(Scheme)
	SchemeBuilder      = runtime.NewSchemeBuilder(addKnownTypes)
	AddToScheme        = SchemeBuilder.AddToScheme
	SchemeGroupVersion = schema.GroupVersion{Group: GroupName, Version: GroupVersion}
)

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion)
	return nil
}

func init() {
	utilruntime.Must(AddToScheme(Scheme))

	// Setting VersionPriority is critical in the InstallAPIGroup call (done in New())
	utilruntime.Must(Scheme.SetVersionPriority(SchemeGroupVersion))

	// TODO(devdattakulkarni) -- Following comments coming from sample-apiserver.
	// Leaving them for now.
	// we need to add the options to empty v1
	// TODO fix the server code to avoid this
	metav1.AddToGroupVersion(Scheme, schema.GroupVersion{Version: GroupVersion})

	// TODO(devdattakulkarni) -- Following comments coming from sample-apiserver.
	// Leaving them for now.
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

	installCompositionProvenanceWebService(s)

	return s, nil
}

func installCompositionProvenanceWebService(provenanceServer *ProvenanceServer) {
	for _, resourceKindPlural := range provenance.KindPluralMap {
		namespaceToUse := provenance.Namespace
		path := "/apis/" + GroupName + "/" + GroupVersion + "/namespaces/"
		path = path + namespaceToUse + "/" + strings.ToLower(resourceKindPlural)
		fmt.Println("WS PATH:" + path)
		ws := getWebService()
		ws.Path(path).
			Consumes(restful.MIME_JSON, restful.MIME_XML).
			Produces(restful.MIME_JSON, restful.MIME_XML)
		getPath := "/{resource-id}/versions"
		fmt.Println("Get Path:" + getPath)
		ws.Route(ws.GET(getPath).To(getVersions))

		historyPath := "/{resource-id}/spechistory"
		fmt.Println("History Path:" + historyPath)
		ws.Route(ws.GET(historyPath).To(getHistory))

		diffPath := "/{resource-id}/diff"
		fmt.Println("Diff Path:" + diffPath)
		ws.Route(ws.GET(diffPath).To(getDiff))

		bisectPath := "/{resource-id}/bisect"
		fmt.Println("Bisect Path:" + bisectPath)
		ws.Route(ws.GET(bisectPath).To(bisect))

		provenanceServer.GenericAPIServer.Handler.GoRestfulContainer.Add(ws)

	}
	fmt.Println("Done registering.")
}

func getWebService() *restful.WebService {
	ws := new(restful.WebService)
	ws.Path("/apis")
	ws.Consumes("*/*")
	ws.Produces(restful.MIME_JSON, restful.MIME_XML)
	ws.ApiVersion(GroupName)
	return ws
}

func getVersions(request *restful.Request, response *restful.Response) {
	resourceName := request.PathParameter("resource-id")
	requestPath := request.Request.URL.Path
	resourcePathSlice := strings.Split(requestPath, "/")
	resourceKind := resourcePathSlice[6] // Kind is 7th element in the slice
	provenanceInfo := "Resource Name:" + resourceName + " Resource Kind:" + resourceKind
	fmt.Println(provenanceInfo)

	response.Write([]byte(provenanceInfo))
	intendedProvObj := provenance.FindProvenanceObjectByName(resourceName, provenance.AllProvenanceObjects)

	//TODO: Validate request based on the correct namespace and the correct plural type. right now
	//deployments is sort of just ignored intentionally. I have the namespace/pluralkind data
	//in my ProvenanceOfObject struct so it is easy to make these changes later
	if intendedProvObj == nil {
		s := fmt.Sprintf("Could not find any provenance history for resource name: %s", resourceName)
		response.Write([]byte(s))
	} else {
		response.Write([]byte(intendedProvObj.ObjectFullHistory.GetVersions()))
	}
}

func getHistory(request *restful.Request, response *restful.Response) {
	fmt.Println("Inside getHistory")
	resourceName := request.PathParameter("resource-id")
	requestPath := request.Request.URL.Path
	resourcePathSlice := strings.Split(requestPath, "/")
	resourceKind := resourcePathSlice[6] // Kind is 7th element in the slice

	provenanceInfo := "Resource Name:" + resourceName + " Resource Kind:" + resourceKind + "\n"
	response.Write([]byte(provenanceInfo))
	intendedProvObj := provenance.FindProvenanceObjectByName(resourceName, provenance.AllProvenanceObjects)

	//TODO: Validate request based on the correct namespace and the correct plural type. right now
	//deployments is sort of just ignored intentionally. I have the namespace/pluralkind data
	//in my ProvenanceOfObject struct so it is easy to make these changes later
	if intendedProvObj == nil {
		s := fmt.Sprintf("Could not find any provenance history for resource name: %s", resourceName)
		response.Write([]byte(s))
	} else {
		response.Write([]byte(intendedProvObj.ObjectFullHistory.SpecHistory()))
	}

}

func bisect(request *restful.Request, response *restful.Response) {
	fmt.Println("Inside bisect")
	resourceName := request.PathParameter("resource-id")
	requestPath := request.Request.URL.Path
	resourcePathSlice := strings.Split(requestPath, "/")
	resourceKind := resourcePathSlice[6] // Kind is 7th element in the slice

	var provenanceInfo string
	provenanceInfo = "Resource Name:" + resourceName + " Resource Kind:" + resourceKind
	fmt.Println(provenanceInfo)

	field := request.QueryParameter("field")
	value := request.QueryParameter("value")

	provenanceInfo = provenanceInfo + " Field:" + field + " Value:" + value

	fmt.Printf("ProvenanceInfo:%v", provenanceInfo)

	response.Write([]byte(provenanceInfo))
}

func getDiff(request *restful.Request, response *restful.Response) {
	fmt.Println("Inside getDiff")
	resourceName := request.PathParameter("resource-id")
	requestPath := request.Request.URL.Path

	// Path looks as follows:
	// /apis/kubeprovenance.cloudark.io/v1/namespaces/default/deployments/dep1/compositions
	resourcePathSlice := strings.Split(requestPath, "/")
	resourceKind := resourcePathSlice[6] // Kind is 7th element in the slice

	fmt.Printf("Resource Name:%s, Resource Kind:%s", resourceName, resourceKind)

	start := request.QueryParameter("start")
	end := request.QueryParameter("end")
	field := request.QueryParameter("field")
	intendedProvObj := provenance.FindProvenanceObjectByName(resourceName, provenance.AllProvenanceObjects)

	if intendedProvObj == nil {
		s := fmt.Sprintf("Could not find any provenance history for resource name: %s", resourceName)
		response.Write([]byte(s))
		return
	}

	var diffInfo string
	if start == "" || end == "" {
		fmt.Printf("Start:%s", start)
		fmt.Printf("End:%s", end)
		diffInfo = "Start and end query parameters missing\n"
	} else {
		fmt.Printf("Start:%s", start)
		fmt.Printf("End:%s", end)
		startInt, err := strconv.Atoi(start)

		if err != nil {
			s := fmt.Sprintf("Could not parse start query parameter to int: %s", err.Error())
			response.Write([]byte(s))
			return
		}
		endInt, err := strconv.Atoi(end)
		if err != nil {
			s := fmt.Sprintf("Could not parse end query parameter to int: %s", err.Error())
			response.Write([]byte(s))
			return
		}

		if field != "" {
			fmt.Printf("Diff for Field requested. Field:%s", field)
			diffInfo = intendedProvObj.ObjectFullHistory.FieldDiff(field, startInt, endInt)
		} else {
			fmt.Println("Diff for Spec requested.")
			diffInfo = intendedProvObj.ObjectFullHistory.FullDiff(startInt, endInt)
		}
	}
	response.Write([]byte(diffInfo))
}

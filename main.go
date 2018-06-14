package main

import (
	"flag"
	"os"

	"github.com/golang/glog"

	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/apiserver/pkg/util/logs"
	"github.com/cloud-ark/kubeprovenance/pkg/cmd/server"
)

func main() {
	logs.InitLogs()
	defer logs.FlushLogs()

	stopCh := genericapiserver.SetupSignalHandler()
	options := server.NewProvenanceServerOptions(os.Stdout, os.Stderr)
	cmd := server.NewCommandStartProvenanceServer(options, stopCh)
	cmd.Flags().AddGoFlagSet(flag.CommandLine)
	if err := cmd.Execute(); err != nil {
		glog.Fatal(err)
	}
}

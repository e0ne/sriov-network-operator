package main

import (
	"flag"
	"os"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
)

const (
	componentName = "sriov-config-service"
)

var (
	rootCmd = &cobra.Command{
		Use:   componentName,
		Short: "Run SR-IOV configuration servicve",
		Long:  "",
	}
)

func init() {
	rootCmd.PersistentFlags().AddGoFlagSet(flag.CommandLine)
}

func main() {
	flag.Set("logtostderr", "true")
	flag.Parse()
	glog.Info("Run sriov-config-service")

	if err := rootCmd.Execute(); err != nil {
		glog.Exitf("Error executing sriov-config-service: %v", err)
		os.Exit(1)
	}
}

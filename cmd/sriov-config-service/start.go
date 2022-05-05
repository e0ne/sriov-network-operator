package main

import (
	"flag"
	"github.com/golang/glog"
	"github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/utils"
	"github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/version"
	"github.com/spf13/cobra"
)

var (
	startCmd = &cobra.Command{
		Use:   "start",
		Short: "Starts SR-IOV Network Config Daemon",
		Long:  "",
		Run:   runStartCmd,
	}

	startOpts struct {
		kubeconfig string
		nodeName   string
	}
)

func init() {
	rootCmd.AddCommand(startCmd)
	startCmd.PersistentFlags().StringVar(&startOpts.kubeconfig, "kubeconfig", "", "Kubeconfig file to access a remote cluster (testing only)")
	startCmd.PersistentFlags().StringVar(&startOpts.nodeName, "node-name", "", "kubernetes node name daemon is managing.")
}

func runStartCmd(cmd *cobra.Command, args []string) {
	flag.Set("logtostderr", "true")
	flag.Set("stderrthreshold", "INFO")
	flag.Parse()

	// To help debugging, immediately log version
	glog.V(2).Infof("Version: %+v", version.Version)

	glog.V(0).Info("Starting sriov-config-service")
	sriovConfig, err := utils.ReadSriovConfFile(utils.SriovHostConfPath)
	glog.V(2).Infof("sriov-config-service read config: %v", sriovConfig)
	glog.V(2).Infof("sriov-config-service read config: %v", sriovConfig[0].VfGroups)
	if err != nil {
		glog.Errorf("WriteSwitchdevConfFile(): fail to read file: %v", err)
		return
	}

	// TODO: implement OpenStack support
	ifaceStatuses, err := utils.DiscoverSriovDevices(false)
	if err != nil {
		glog.V(0).Infof("sriov-config-service failed %v", err)
		return
	} else {
		if err := utils.ConfigSriovInterfaces(sriovConfig, ifaceStatuses); err != nil {
			glog.V(0).Infof("sriov-config-service failed %v", err)
		}
	}
	glog.V(0).Info("Shutting down sriov-config-service")
}

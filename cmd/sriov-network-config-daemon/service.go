package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/golang/glog"
	sriovv1 "github.com/k8snetworkplumbingwg/sriov-network-operator/api/v1"
	"github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/host"
	plugin "github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/plugins"
	"github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/plugins/generic"
	"github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/systemd"
	"github.com/spf13/cobra"
	"os"

	"github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/utils"
	"github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/version"
)

var (
	serviceCmd = &cobra.Command{
		Use:   "service",
		Short: "Starts SR-IOV service Config",
		Long:  "",
		Run:   runServiceCmd,
	}

	serviceOpts struct {
		kubeconfig string
		nodeName   string
	}
)

func init() {
	rootCmd.AddCommand(serviceCmd)
	serviceCmd.PersistentFlags().StringVar(&startOpts.kubeconfig, "kubeconfig", "", "Kubeconfig file to access a remote cluster (testing only)")
	serviceCmd.PersistentFlags().StringVar(&startOpts.nodeName, "node-name", "", "kubernetes node name daemon is managing.")
}

func runServiceCmd(cmd *cobra.Command, args []string) {
	flag.Set("logtostderr", "true")
	flag.Set("stderrthreshold", "INFO")
	flag.Parse()

	// To help debugging, immediately log version
	glog.V(2).Infof("Version: %+v", version.Version)

	glog.V(0).Info("Starting sriov-config-service")
	supportedNicIds, err := systemd.ReadSriovSupportedNics()
	if err != nil {
		glog.Errorf("failed to read list of supported nic ids")
		sriovResult := &systemd.SriovResult{
			SyncStatus:    "Failed",
			LastSyncError: fmt.Sprintf("failed to read list of supported nic ids: %v", err),
		}
		err = systemd.WriteSriovResult(sriovResult)
		if err != nil {
			glog.Errorf("sriov-config-service failed to write sriov result file with content %v error: %v", *sriovResult, err)
			return
		}
		return
	}
	sriovv1.InitNicIDMapFromList(supportedNicIds)

	nodeStateSpec, err := systemd.ReadConfFile()
	if err != nil {
		if _, err := os.Stat(utils.SriovSwitchDevConfPath); !errors.Is(err, os.ErrNotExist) {
			glog.Errorf("failed to read the sriov configuration file in path %s: %v", utils.SriovSwitchDevConfPath, err)
			sriovResult := &systemd.SriovResult{
				SyncStatus:    "Failed",
				LastSyncError: fmt.Sprintf("failed to read the sriov configuration file in path %s: %v", utils.SriovSwitchDevConfPath, err),
			}
			err = systemd.WriteSriovResult(sriovResult)
			if err != nil {
				glog.Errorf("sriov-config-service failed to write sriov result file with content %v error: %v", *sriovResult, err)
				return
			}
		}

		nodeStateSpec = &systemd.SriovConfig{
			Spec:            sriovv1.SriovNetworkNodeStateSpec{},
			UnsupportedNics: false,
			PlatformType:    utils.Baremetal,
		}
	}

	glog.V(2).Infof("sriov-config-service read config: %v", nodeStateSpec)

	// Load kernel modules
	hostManager := host.NewHostManager(true)
	_, err = hostManager.TryEnableRdma()
	if err != nil {
		glog.Warningf("failed to enable RDMA: %v", err)
	}
	hostManager.TryEnableTun()
	hostManager.TryEnableVhostNet()

	var configPlugin plugin.VendorPlugin
	var ifaceStatuses []sriovv1.InterfaceExt
	if nodeStateSpec.PlatformType == utils.Baremetal {
		// Bare metal support
		ifaceStatuses, err = utils.DiscoverSriovDevices(nodeStateSpec.UnsupportedNics)
		if err != nil {
			glog.V(0).Infof("sriov-config-service failed %v", err)
			return
		}

		// Create the generic plugin
		configPlugin, err = generic.NewGenericPlugin(true)
		if err != nil {
			glog.Errorf("sriov-config-service failed to create generic plugin %v", err)
			return
		}

	} else if nodeStateSpec.PlatformType == utils.VirtualOpenStack {
		// Openstack support
		metaData, networkData, err := utils.GetOpenstackData(false)
		if err != nil {
			glog.Errorf("RunOnce(): failed to read OpenStack data: %v", err)
			return
		}

		openStackDevicesInfo, err := utils.CreateOpenstackDevicesInfo(metaData, networkData)
		if err != nil {
			glog.Errorf("RunOnce(): failed to read OpenStack data: %v", err)
			return
		}

		ifaceStatuses, err = utils.DiscoverSriovDevicesVirtual(openStackDevicesInfo)
		if err != nil {
			glog.Errorf("RunOnce(): failed to read OpenStack data: %v", err)
			return
		}
	}

	nodeState := &sriovv1.SriovNetworkNodeState{
		Spec:   nodeStateSpec.Spec,
		Status: sriovv1.SriovNetworkNodeStateStatus{Interfaces: ifaceStatuses},
	}

	// Set "In Progress" state by to indicate work is started
	sriovResult := &systemd.SriovResult{
		SyncStatus:    "In Progress",
		LastSyncError: "",
	}

	err = systemd.WriteSriovResult(sriovResult)
	if err != nil {
		glog.Errorf("sriov-config-service failed to write sriov result file with content %v error: %v", *sriovResult, err)
		return
	}

	_, _, err = configPlugin.OnNodeStateChange(nodeState)
	if err != nil {
		glog.Errorf("sriov-config-service failed to run OnNodeStateChange to update the generic plugin status %v", err)
		return
	}

	sriovResult = &systemd.SriovResult{
		SyncStatus:    "Succeeded",
		LastSyncError: "",
	}

	err = configPlugin.Apply()
	if err != nil {
		glog.Errorf("sriov-config-service failed to run apply node configuration %v", err)
		sriovResult.SyncStatus = "Failed"
		sriovResult.LastSyncError = err.Error()
	}

	err = systemd.WriteSriovResult(sriovResult)
	if err != nil {
		glog.Errorf("sriov-config-service failed to write sriov result file with content %v error: %v", *sriovResult, err)
		return
	}

	glog.V(0).Info("Shutting down sriov-config-service")
}

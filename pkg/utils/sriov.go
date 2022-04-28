package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/golang/glog"

	sriovnetworkv1 "github.com/k8snetworkplumbingwg/sriov-network-operator/api/v1"
)

const (
	SriovConfPath     = "/etc/sriov_config.json"
	SriovHostConfPath = "/host" + SriovConfPath
)

type config struct {
	Interfaces []sriovnetworkv1.Interface `json:"interfaces"`
}

func IsSwitchdevModeSpec(spec sriovnetworkv1.SriovNetworkNodeStateSpec) bool {
	for _, iface := range spec.Interfaces {
		if iface.EswitchMode == sriovnetworkv1.ESwithModeSwitchDev {
			return true
		}
	}
	return false
}

func ReadSriovConfFile(configPath string) (interfaces []sriovnetworkv1.Interface, err error) {
	rawConfig, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	cfg := config{}
	json.Unmarshal(rawConfig, &cfg)

	return cfg.Interfaces, nil
}

func findInterface(interfaces sriovnetworkv1.Interfaces, name string) (iface sriovnetworkv1.Interface, err error) {
	for _, i := range interfaces {
		if i.Name == name {
			return i, nil
		}
	}
	return sriovnetworkv1.Interface{}, fmt.Errorf("unable to find interface: %v", name)
}

func WriteSwitchdevConfFile(newState *sriovnetworkv1.SriovNetworkNodeState, configPath string) (update bool, err error) {
	cfg := config{}
	for _, iface := range newState.Spec.Interfaces {
		for _, ifaceStatus := range newState.Status.Interfaces {
			if iface.PciAddress != ifaceStatus.PciAddress {
				continue
			}
			if !SkipConfigVf(iface, ifaceStatus) {
				continue
			}
			i := sriovnetworkv1.Interface{}
			if iface.NumVfs > 0 {
				var vfGroups []sriovnetworkv1.VfGroup = nil
				ifc, err := findInterface(newState.Spec.Interfaces, iface.Name)
				if err != nil {
					glog.Errorf("WriteSwitchdevConfFile(): fail find interface: %v", err)
				} else {
					vfGroups = ifc.VfGroups
				}
				i = sriovnetworkv1.Interface{
					// Not passing all the contents, since only NumVfs and EswitchMode can be configured by configure-switchdev.sh currently.
					Name:       iface.Name,
					PciAddress: iface.PciAddress,
					NumVfs:     iface.NumVfs,
					Mtu:        iface.Mtu,
					VfGroups:   vfGroups,
				}

				if iface.EswitchMode == sriovnetworkv1.ESwithModeSwitchDev {
					i.EswitchMode = iface.EswitchMode
				}
				cfg.Interfaces = append(cfg.Interfaces, i)
			}
		}
	}
	_, err = os.Stat(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			if len(cfg.Interfaces) == 0 {
				return
			}
			glog.V(2).Infof("WriteSwitchdevConfFile(): file not existed, create it")
			_, err = os.Create(configPath)
			if err != nil {
				glog.Errorf("WriteSwitchdevConfFile(): fail to create file: %v", err)
				return
			}
		} else {
			return
		}
	}
	oldContent, err := ioutil.ReadFile(configPath)
	if err != nil {
		glog.Errorf("WriteSwitchdevConfFile(): fail to read file: %v", err)
		return
	}
	var newContent []byte
	if len(cfg.Interfaces) != 0 {
		newContent, err = json.Marshal(cfg)
		if err != nil {
			glog.Errorf("WriteSwitchdevConfFile(): fail to marshal config: %v", err)
			return
		}
	}

	if bytes.Equal(newContent, oldContent) {
		glog.V(2).Info("WriteSwitchdevConfFile(): no update")
		return
	}
	update = true
	glog.V(2).Infof("WriteSwitchdevConfFile(): write '%s' to switchdev.conf", newContent)
	err = ioutil.WriteFile(configPath, []byte(newContent), 0644)
	if err != nil {
		glog.Errorf("WriteSwitchdevConfFile(): fail to write file: %v", err)
		return
	}
	return
}

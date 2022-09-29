package systemd

import (
	"bytes"
	"errors"
	"io/ioutil"
	"os"

	"github.com/golang/glog"
	"gopkg.in/yaml.v3"

	sriovnetworkv1 "github.com/k8snetworkplumbingwg/sriov-network-operator/api/v1"
	"github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/utils"
)

const (
	SriovSystemdConfigPath     = utils.SriovConfBasePath + "/sriov-interface-config.yaml"
	SriovSystemdResultPath     = utils.SriovConfBasePath + "/sriov-interface-result.yaml"
	SriovHostSystemdConfigPath = "/host" + SriovSystemdConfigPath
	SriovHostSystemdResultPath = "/host" + SriovSystemdResultPath
)

var (
	systemdModeEnabled = false
)

type SriovConfig struct {
	Generation int64                                    `yaml:"generation"`
	Spec       sriovnetworkv1.SriovNetworkNodeStateSpec `yaml:"spec"`
}

type SriovResult struct {
	Generation    int64  `yaml:"generation"`
	SyncStatus    string `yaml:"syncStatus"`
	LastSyncError string `yaml:"lastSyncError"`
}

func ReadConfFile() (spec *SriovConfig, err error) {
	rawConfig, err := ioutil.ReadFile(SriovSystemdConfigPath)
	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal(rawConfig, &spec)

	return spec, err
}

func WriteConfFile(newState *sriovnetworkv1.SriovNetworkNodeState) (bool, error) {
	update := false
	sriovConfig := &SriovConfig{
		newState.Generation,
		newState.Spec,
	}

	_, err := os.Stat(SriovSystemdConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			if len(sriovConfig.Spec.Interfaces) == 0 {
				err = nil
				return false, err
			}

			// Create the sriov-operator folder on the host if it doesn't exist
			if _, err := os.Stat("/host" + utils.SriovConfBasePath); os.IsNotExist(err) {
				err = os.Mkdir("/host"+utils.SriovConfBasePath, os.ModeDir)
				if err != nil {
					glog.Errorf("WriteConfFile(): fail to create sriov-operator folder: %v", err)
					return false, err
				}
			}

			glog.V(2).Infof("WriteConfFile(): file not existed, create it")
			_, err = os.Create(SriovHostSystemdConfigPath)
			if err != nil {
				glog.Errorf("WriteConfFile(): fail to create file: %v", err)
				return false, err
			}
			update = true
		} else {
			return update, err
		}
	}
	oldContent, err := ioutil.ReadFile(SriovHostSystemdConfigPath)
	if err != nil {
		glog.Errorf("WriteConfFile(): fail to read file: %v", err)
		return update, err
	}
	var newContent []byte
	if len(sriovConfig.Spec.Interfaces) != 0 {
		newContent, err = yaml.Marshal(sriovConfig)
		if err != nil {
			glog.Errorf("WriteConfFile(): fail to marshal config: %v", err)
			return update, err
		}
	} else {
		err := os.Remove(SriovHostSystemdConfigPath)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			glog.Errorf("WriteConfFile(): fail to remove the config file: %v", err)
			return update, err
		}

		err = os.Remove(SriovHostSystemdResultPath)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			glog.Errorf("WriteConfFile(): fail to remove the result file: %v", err)
			return update, err
		}
	}

	if bytes.Equal(newContent, oldContent) {
		glog.V(2).Info("WriteConfFile(): no update")
		return update, err
	}

	update = true
	glog.V(2).Infof("WriteConfFile(): write '%s' to %s", newContent, SriovHostSystemdConfigPath)
	err = ioutil.WriteFile(SriovHostSystemdConfigPath, newContent, 0644)
	if err != nil {
		glog.Errorf("WriteConfFile(): fail to write file: %v", err)
		return update, err
	}
	return update, err
}

func WriteSriovResult(result *SriovResult) error {
	_, err := os.Stat(SriovSystemdResultPath)
	if err != nil {
		if os.IsNotExist(err) {
			glog.V(2).Infof("WriteSriovResult(): file not existed, create it")
			_, err = os.Create(SriovSystemdResultPath)
			if err != nil {
				glog.Errorf("WriteSriovResult(): fail to create sriov result file on path %s: %v", SriovSystemdResultPath, err)
				return err
			}
		} else {
			glog.Errorf("WriteSriovResult(): fail to check sriov result file on path %s: %v", SriovSystemdResultPath, err)
			return err
		}
	}

	out, err := yaml.Marshal(result)
	if err != nil {
		glog.Errorf("WriteSriovResult(): fail to marshal sriov result file: %v", err)
		return err
	}

	glog.V(2).Infof("WriteConfFile(): write '%s' to %s", string(out), SriovSystemdResultPath)
	err = ioutil.WriteFile(SriovSystemdResultPath, out, 0644)
	if err != nil {
		glog.Errorf("WriteConfFile(): fail to write sriov result file on path %s: %v", SriovSystemdResultPath, err)
		return err
	}

	return nil
}

func ReadSriovResult() (*SriovResult, error) {
	_, err := os.Stat(SriovHostSystemdResultPath)
	if err != nil {
		if os.IsNotExist(err) {
			glog.V(2).Infof("ReadSriovResult(): file not existed, return empty result")
			return &SriovResult{}, err
		} else {
			glog.Errorf("ReadSriovResult(): failed to check sriov result file on path %s: %v", SriovHostSystemdResultPath, err)
			return nil, err
		}
	}

	rawConfig, err := ioutil.ReadFile(SriovHostSystemdResultPath)
	if err != nil {
		glog.Errorf("ReadSriovResult(): failed to read sriov result file on path %s: %v", SriovHostSystemdResultPath, err)
		return nil, err
	}

	result := &SriovResult{}
	err = yaml.Unmarshal(rawConfig, &result)
	if err != nil {
		glog.Errorf("ReadSriovResult(): failed to unmarshal sriov result file on path %s: %v", SriovHostSystemdResultPath, err)
		return nil, err
	}
	return result, err
}

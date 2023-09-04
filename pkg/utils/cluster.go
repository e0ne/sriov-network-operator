package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/consts"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/kubernetes"
	"os"

	"github.com/golang/glog"

	configv1 "github.com/openshift/api/config/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// default Infrastructure resource name for Openshift
	infraResourceName        = "cluster"
	workerRoleName           = "worker"
	masterRoleName           = "master"
	workerNodeLabelKey       = "node-role.kubernetes.io/worker"
	masterNodeLabelKey       = "node-role.kubernetes.io/master"
	controlPlaneNodeLabelKey = "node-role.kubernetes.io/control-plane"
)

func getNodeRole(node corev1.Node) string {
	for k := range node.Labels {
		if k == workerNodeLabelKey {
			return workerRoleName
		} else if k == masterNodeLabelKey || k == controlPlaneNodeLabelKey {
			return masterRoleName
		}
	}
	return ""
}

func IsSingleNodeCluster(c client.Client) (bool, error) {
	if os.Getenv("CLUSTER_TYPE") == ClusterTypeOpenshift {
		topo, err := openshiftControlPlaneTopologyStatus(c)
		if err != nil {
			return false, err
		}
		if topo == configv1.SingleReplicaTopologyMode {
			return true, nil
		}
		return false, nil
	}
	return k8sSingleNodeClusterStatus(c)
}

// IsExternalControlPlaneCluster detects control plane location of the cluster.
// On OpenShift, the control plane topology is configured in configv1.Infrastucture struct.
// On kubernetes, it is determined by which node the sriov operator is scheduled on. If operator
// pod is schedule on worker node, it is considered as external control plane.
func IsExternalControlPlaneCluster(c client.Client) (bool, error) {
	if os.Getenv("CLUSTER_TYPE") == ClusterTypeOpenshift {
		topo, err := openshiftControlPlaneTopologyStatus(c)
		if err != nil {
			return false, err
		}
		if topo == "External" {
			return true, nil
		}
	} else if os.Getenv("CLUSTER_TYPE") == ClusterTypeKubernetes {
		role, err := operatorNodeRole(c)
		if err != nil {
			return false, err
		}
		if role == workerRoleName {
			return true, nil
		}
	}
	return false, nil
}

func k8sSingleNodeClusterStatus(c client.Client) (bool, error) {
	nodeList := &corev1.NodeList{}
	err := c.List(context.TODO(), nodeList)
	if err != nil {
		glog.Errorf("k8sSingleNodeClusterStatus(): Failed to list nodes: %v", err)
		return false, err
	}

	if len(nodeList.Items) == 1 {
		glog.Infof("k8sSingleNodeClusterStatus(): one node found in the cluster")
		return true, nil
	}
	return false, nil
}

// operatorNodeRole returns role of the node where operator is scheduled on
func operatorNodeRole(c client.Client) (string, error) {
	node := corev1.Node{}
	err := c.Get(context.TODO(), types.NamespacedName{Name: os.Getenv("NODE_NAME")}, &node)
	if err != nil {
		glog.Errorf("k8sIsExternalTopologyMode(): Failed to get node: %v", err)
		return "", err
	}

	return getNodeRole(node), nil
}

func openshiftControlPlaneTopologyStatus(c client.Client) (configv1.TopologyMode, error) {
	infra := &configv1.Infrastructure{}
	err := c.Get(context.TODO(), types.NamespacedName{Name: infraResourceName}, infra)
	if err != nil {
		return "", fmt.Errorf("openshiftControlPlaneTopologyStatus(): Failed to get Infrastructure (name: %s): %v", infraResourceName, err)
	}
	if infra == nil {
		return "", fmt.Errorf("openshiftControlPlaneTopologyStatus(): getting resource Infrastructure (name: %s) succeeded but object was nil", infraResourceName)
	}
	return infra.Status.ControlPlaneTopology, nil
}

func NodeHasAnnotation(node corev1.Node, annoKey string, value string) bool {
	// Check if node already contains annotation
	if anno, ok := node.Annotations[annoKey]; ok && (anno == value) {
		return true
	}
	return false
}

func AnnotateNode(node, value string, kubeClient kubernetes.Interface) error {
	glog.Infof("annotateNode(): Annotate node %s with: %s", node, value)
	oldNode, err := kubeClient.CoreV1().Nodes().Get(context.Background(), node, metav1.GetOptions{})
	if err != nil {
		glog.Infof("annotateNode(): Failed to get node %s %v, retrying", node, err)
		return err
	}

	oldData, err := json.Marshal(oldNode)
	if err != nil {
		return err
	}

	newNode := oldNode.DeepCopy()
	if newNode.Annotations == nil {
		newNode.Annotations = map[string]string{}
	}

	if newNode.Annotations[consts.NodeDrainAnnotation] != value {
		newNode.Annotations[consts.NodeDrainAnnotation] = value
		newData, err := json.Marshal(newNode)
		if err != nil {
			return err
		}
		patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldData, newData, corev1.Node{})
		if err != nil {
			return err
		}
		_, err = kubeClient.CoreV1().Nodes().Patch(context.Background(),
			node,
			types.StrategicMergePatchType,
			patchBytes,
			metav1.PatchOptions{})
		if err != nil {
			glog.Infof("annotateNode(): Failed to patch node %s: %v", node, err)
			return err
		}
	}
	return nil
}

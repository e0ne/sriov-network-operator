package controllers

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "github.com/k8snetworkplumbingwg/sriov-network-operator/api/v1"
	snclientset "github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/client/clientset/versioned"
	constants "github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/consts"
)

type MigrationReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	ClientSet snclientset.Interface
}

//+kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch;create;update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (mr *MigrationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	req.Namespace = namespace
	reqLogger := log.FromContext(ctx).WithValues("drain", req.NamespacedName)
	reqLogger.Info("Reconciling annotation migration")

	node := &corev1.Node{}
	err := mr.Get(ctx, types.NamespacedName{
		Name: req.Name}, node)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		reqLogger.Error(err, "Error occurred on GET SriovOperatorConfig request from API server.")
		return reconcile.Result{}, err
	}

	if anno, ok := node.Annotations[constants.NodeDrainAnnotation]; ok {
		nodeState := &v1.SriovNetworkNodeState{}
		err := mr.Get(ctx, types.NamespacedName{
			Name: node.Name, Namespace: namespace}, nodeState)
		if err != nil {
			reqLogger.Error(err, "Error occurred on GET SriovNetworkNodeState request from API server.")
			return reconcile.Result{}, err
		}
		nodeState.Status.DrainStatus = anno
		mr.ClientSet.SriovnetworkV1().SriovNetworkNodeStates(namespace).UpdateStatus(context.TODO(), nodeState, metav1.UpdateOptions{})
		delete(node.Annotations, constants.NodeDrainAnnotation)
		mr.Update(ctx, node)
		//patch := []byte(fmt.Sprintf(`{"status":{"drainStatus":"%s"}}`, anno))
		//err = mr.Client.Patch(context.TODO(), nodeState, client.RawPatch(types.StrategicMergePatchType, patch))
		//if err != nil {
		//	reqLogger.Error(err, "Error occurred on SriovNetworkNodeState update.")
		//	return reconcile.Result{}, err
		//}

	}

	return reconcile.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (mr *MigrationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	nodePredicates := builder.WithPredicates(DrainAnnotationPredicate{})

	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Node{}, nodePredicates).
		Complete(mr)
}

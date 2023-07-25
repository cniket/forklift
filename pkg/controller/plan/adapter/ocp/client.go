package ocp

import (
	"context"

	planapi "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/plan"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/konveyor/forklift-controller/pkg/controller/plan/context"
	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
	core "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cnv "kubevirt.io/api/core/v1"
	export "kubevirt.io/api/export/v1alpha1"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Client struct {
	*plancontext.Context
}

// CheckSnapshotReady implements base.Client
func (r *Client) CheckSnapshotReady(vmRef ref.Ref, snapshot string) (bool, error) {
	return false, nil
}

// Close implements base.Client
func (r *Client) Close() {
	// NOOP for OCP
}

// CreateSnapshot implements base.Client
func (r *Client) CreateSnapshot(vmRef ref.Ref) (string, error) {
	return "", nil
}

// Finalize implements base.Client
func (r *Client) Finalize(vms []*planapi.VMStatus, planName string) {
	for _, vm := range vms {
		vmExport := &export.VirtualMachineExport{ObjectMeta: v1.ObjectMeta{
			Name:      vm.Name,
			Namespace: vm.Namespace,
		}}

		err := r.Client.Delete(context.TODO(), vmExport)
		if err != nil {
			r.Log.Info("Failed to delete VMExport", "VMExport", vmExport, "Error", err)
			continue
		}
	}
}

// PowerOff implements base.Client
func (r *Client) PowerOff(vmRef ref.Ref) error {
	vm := cnv.VirtualMachine{}
	err := r.Client.Get(context.TODO(), client.ObjectKey{Namespace: vmRef.Namespace, Name: vmRef.Name}, &vm)
	if err != nil {
		return err
	}

	running := false
	vm.Spec.Running = &running
	err = r.Client.Update(context.Background(), &vm)
	if err != nil {
		return err
	}

	return nil
}

// PowerOn implements base.Client
func (r *Client) PowerOn(vmRef ref.Ref) error {
	vm := cnv.VirtualMachine{}
	err := r.Client.Get(context.TODO(), client.ObjectKey{Namespace: vmRef.Namespace, Name: vmRef.Name}, &vm)
	if err != nil {
		return err
	}

	running := true
	vm.Spec.Running = &running
	err = r.Client.Update(context.Background(), &vm)
	if err != nil {
		return err
	}

	return nil
}

// PowerState implements base.Client
func (r *Client) PowerState(vmRef ref.Ref) (string, error) {
	vm := cnv.VirtualMachine{}
	err := r.Client.Get(context.TODO(), client.ObjectKey{Namespace: vmRef.Namespace, Name: vmRef.Name}, &vm)
	if err != nil {
		err = liberr.Wrap(err)
		return "", err
	}

	if vm.Spec.Running != nil && *vm.Spec.Running {
		return "On", nil
	}

	return "Off", nil
}

// PoweredOff implements base.Client
func (r *Client) PoweredOff(vmRef ref.Ref) (bool, error) {
	vm := cnv.VirtualMachine{}
	err := r.Client.Get(context.TODO(), client.ObjectKey{Namespace: vmRef.Namespace, Name: vmRef.Name}, &vm)
	if err != nil {
		err = liberr.Wrap(err)
		return false, err
	}

	if vm.Spec.Running != nil && *vm.Spec.Running {
		return false, nil
	}

	return true, nil
}

// RemoveSnapshots implements base.Client
func (r *Client) RemoveSnapshots(vmRef ref.Ref, precopies []planapi.Precopy) error {
	return nil
}

// SetCheckpoints implements base.Client
func (r *Client) SetCheckpoints(vmRef ref.Ref, precopies []planapi.Precopy, datavolumes []cdi.DataVolume, final bool) (err error) {
	return nil
}

func (r *Client) DetachDisks(vmRef ref.Ref) (err error) {
	// no-op
	return
}

// PreTransferActions implements base.Builder
func (r *Client) PreTransferActions(vmRef ref.Ref) (ready bool, err error) {
	apiGroup := cnv.GroupVersion.Group

	// Check if VM export exists
	vmExport := &export.VirtualMachineExport{}
	err = r.Client.Get(context.Background(), client.ObjectKey{Namespace: vmRef.Namespace, Name: vmRef.Name}, vmExport)
	if err != nil {
		if !k8serr.IsNotFound(err) {
			r.Log.Error(err, "Failed to get VM-export.", "vm", vmRef.Name)
			return true, liberr.Wrap(err)
		}
		// Create VM export
		vmExport = &export.VirtualMachineExport{
			TypeMeta: meta.TypeMeta{
				Kind:       "VirtualMachineExport",
				APIVersion: "kubevirt.io/v1alpha3",
			},
			ObjectMeta: meta.ObjectMeta{
				Name:      vmRef.Name,
				Namespace: vmRef.Namespace,
			},
			Spec: export.VirtualMachineExportSpec{
				Source: core.TypedLocalObjectReference{
					APIGroup: &apiGroup,
					Kind:     "VirtualMachine",
					Name:     vmRef.Name,
				},
			},
		}

		err = r.Client.Create(context.Background(), vmExport, &client.CreateOptions{})
		if err != nil {
			return true, liberr.Wrap(err)
		}
	}
	if vmExport.Status != nil && vmExport.Status.Phase == export.Ready {
		r.Log.Info("VM-export is ready.", "vm", vmRef.Name)
		return true, nil
	}

	r.Log.Info("Waiting for VM-export to be ready...", "vm", vmRef.Name)
	return false, nil
}

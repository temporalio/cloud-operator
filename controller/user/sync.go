package user

import (
	"context"
	"errors"
	"fmt"

	ctrlerrors "github.com/temporalio/kube-temporal/pkg/controller/errors"
	ctrlreqlog "github.com/temporalio/kube-temporal/pkg/controller/request/log"
	"github.com/temporalio/kube-temporal/pkg/controller/requeue"
	kttypes "github.com/temporalio/kube-temporal/pkg/types"
	tcloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	"google.golang.org/protobuf/proto"

	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	"github.com/temporalio/cloud-operator/pkg/resource/reference"
	"github.com/temporalio/cloud-operator/pkg/temporal/cloud"
	"github.com/temporalio/cloud-operator/pkg/types"
)

// ResourceSync creates the Temporal Cloud user if it doesn't exist yet, and
// otherwise updates it to match `desired`. It returns a copy of `latest` with
// any status changes.
//
// `desired` is NEVER mutated.
// `latest` is NEVER mutated.
func (r *Reconciler) ResourceSync(
	ctx context.Context,
	desired kttypes.Resource,
	latest kttypes.Resource,
) (kttypes.Resource, error) {
	var err error
	log := ctrlreqlog.FromContext(ctx)
	exit := log.Trace("resource.sync")
	defer func() {
		exit(err)
	}()

	updated := latest.DeepCopy()

	tcc, err := r.CloudClientFor(ctx, desired)
	if err != nil {
		return updated, fmt.Errorf("failed getting tcloud client: %w", err)
	}

	desiredUser := desired.CR().(*tcloudv1alpha1.User)
	latestUser := latest.CR().(*tcloudv1alpha1.User)

	// Creating or updating the user needs an ID for each ref in the spec.
	if err = checkReferences(desiredUser, latestUser.Status.References); err != nil {
		return updated, err
	}

	switch {
	case !latest.(types.Resource).PreviouslyReconciled():
		err = createUser(ctx, tcc, desiredUser, updated.CR().(*tcloudv1alpha1.User))
		return updated, err
	case latestUser.Status.Resource.State == "":
		// latest.go kept the ID, but GetUser returned NotFound.
		// Only a create that GetUser hasn't returned yet has an operation to wait for.
		if latestUser.Status.Resource.AsyncOperationID == "" {
			err = userGoneError(latestUser.Status.Resource.ID)
		} else {
			err = checkCreate(ctx, tcc, latestUser)
		}
		return updated, err
	}

	err = updateUser(ctx, tcc, desiredUser, latestUser)
	return updated, err
}

// createUser creates the Temporal Cloud user for the supplied `desired` CR and
// saves its ID and async operation ID in `updated`.
//
// `desired` is NEVER mutated.
func createUser(
	ctx context.Context,
	tcc tcloudservicev1.CloudServiceClient,
	desired *tcloudv1alpha1.User,
	updated *tcloudv1alpha1.User, // mutated by this function
) error {
	resp, err := tcc.CreateUser(ctx, &tcloudservicev1.CreateUserRequest{
		Spec: desired.Spec.ToProto(desired.Namespace, updated.Status.References),
	})
	if err != nil {
		return cloud.CallError("creating tcloud user", err)
	}

	// GetUser can return NotFound until the create finishes. The operation ID
	// lets the next reconcile wait instead of creating the user again.
	updated.Status.Resource.ID = resp.GetUserId()
	updated.Status.Resource.AsyncOperationID = resp.GetAsyncOperation().GetId()
	return requeue.NeededAfter(
		fmt.Errorf("created tcloud user %q, waiting for the user to be created", resp.GetUserId()),
		requeue.DefaultRequeueAfterDuration,
	)
}

// checkCreate checks the async operation that createUser saved for a user that
// GetUser didn't find. It requeues while the operation runs, and returns a
// terminal error once the operation failed or finished without the user
// appearing in this account.
func checkCreate(
	ctx context.Context,
	tcc tcloudservicev1.CloudServiceClient,
	latest *tcloudv1alpha1.User,
) error {
	userID := latest.Status.Resource.ID
	opID := latest.Status.Resource.AsyncOperationID

	err := cloud.CheckAsyncOperation(ctx, tcc, opID)
	if errors.Is(err, cloud.ErrAsyncOperationFailed) {
		return fmt.Errorf("creating tcloud user %q failed: %w", userID, err)
	}
	var pending *requeue.RequeueNeededAfter
	if errors.As(err, &pending) {
		return requeue.NeededAfter(
			fmt.Errorf(
				"waiting for tcloud user %q to be created (async operation %s)",
				userID, opID,
			),
			requeue.DefaultRequeueAfterDuration,
		)
	}
	if err != nil {
		return err
	}

	return userGoneError(userID)
}

// userGoneError returns the terminal error for a user that GetUser no longer
// finds in this account. The operator doesn't create the user again, because
// that could re-invite someone an admin removed, or move a user back from
// another account and email them each time.
func userGoneError(userID string) error {
	return ctrlerrors.NewTerminalError(
		fmt.Errorf(
			"%w: tcloud user %q no longer exists in this account. "+
				"It was deleted or invited to another account. "+
				"Delete and re-apply this User CR to invite them again",
			ctrlerrors.ErrResourceSync, userID,
		),
	)
}

// updateUser move an existing user toward the desired state.
func updateUser(
	ctx context.Context,
	tcc tcloudservicev1.CloudServiceClient,
	desired *tcloudv1alpha1.User,
	latest *tcloudv1alpha1.User,
) error {
	resourceID := latest.Status.Resource.ID
	err := cloud.RequireActive(resourceKind, resourceID, latest.Status.Resource.State)
	if err != nil {
		return err
	}

	want := desired.Spec.ToProto(desired.Namespace, latest.Status.References)
	var none tcloudv1alpha1.UserReferences
	got := latest.Spec.ToProto(latest.Namespace, none)

	// Desired can have refs where latest has their resolved IDs (desired has
	// {ref: tco-adopt-test2} where latest has {id: tco-adopt-test2.ACCOUNT_ID}), so compare the
	// protos that would be sent to Temporal Cloud.
	if proto.Equal(want, got) {
		return nil
	}

	// Temporal Cloud refuses to update an account Owner. The spec can't set the
	// Owner role, so the user became an Owner outside of the operator.
	if aa := latest.Spec.Access.AccountAccess; aa != nil && aa.Role == tcloudv1alpha1.AccountRoleOwner {
		return ctrlerrors.NewTerminalError(
			fmt.Errorf(
				"%w: tcloud user %q is an account Owner, which Temporal Cloud doesn't allow to be updated",
				ctrlerrors.ErrResourceSync, resourceID,
			),
		)
	}

	_, err = tcc.UpdateUser(ctx, &tcloudservicev1.UpdateUserRequest{
		UserId:          resourceID,
		Spec:            want,
		ResourceVersion: latest.Status.Resource.Version,
	})
	if err != nil {
		return cloud.CallError("updating tcloud user", err)
	}
	return requeue.NeededAfter(
		fmt.Errorf("updated tcloud user %q, waiting for update to complete", resourceID),
		requeue.DefaultRequeueAfterDuration,
	)
}

// checkReferences checks the refs in the spec before anything is sent to
// Temporal Cloud.
func checkReferences(
	cr *tcloudv1alpha1.User,
	refs tcloudv1alpha1.UserReferences,
) error {
	check := reference.NewCheck(cr.Namespace)
	for _, nr := range namespaceRefs(cr.Spec) {
		id, _ := refs.NamespaceID(*nr.refOrID.Ref, cr.Namespace)
		check.Pending(nr.path, nr.refOrID, id)
	}

	access := cr.Spec.Access
	for i, e := range access.ProjectAccesses {
		check.Unsupported(fmt.Sprintf("spec.access.projectAccesses[%d].project", i), e.Project)
	}
	if access.AccountAccess != nil {
		for i, customRole := range access.AccountAccess.CustomRoles {
			check.Unsupported(fmt.Sprintf("spec.access.accountAccess.customRoles[%d]", i), customRole)
		}
	}
	if err := checkDuplicateNamespaces(cr, refs); err != nil {
		return err
	}
	return check.Err()
}

// checkDuplicateNamespaces returns a terminal error when two namespace accesses
// in the spec refer to the same Temporal Cloud namespace.Ex:
// {ref: payments} and {id: payments.acct1}
func checkDuplicateNamespaces(
	cr *tcloudv1alpha1.User,
	refs tcloudv1alpha1.UserReferences,
) error {
	const path = "spec.access.namespaceAccesses"
	seen := map[string]int{}
	for i, e := range cr.Spec.Access.NamespaceAccesses {
		id := e.Namespace.ID
		if e.Namespace.Ref != nil {
			id, _ = refs.NamespaceID(*e.Namespace.Ref, cr.Namespace)
		}
		if id == "" {
			continue
		}
		if first, ok := seen[id]; ok {
			return ctrlerrors.NewTerminalError(
				fmt.Errorf(
					"%w: %s[%d].namespace and %s[%d].namespace both refer to tcloud namespace %q",
					ctrlerrors.ErrResourceSync, path, first, path, i, id,
				),
			)
		}
		seen[id] = i
	}
	return nil
}

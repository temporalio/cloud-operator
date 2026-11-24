package namespace

import (
	"fmt"
	"strings"

	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	"github.com/temporalio/cloud-operator/pkg/types"
)

// checkAdoptName returns an error if the Namespace that ResourceLatest found
// doesn't have the CR's name.
func checkAdoptName(latest types.Resource) error {
	foundID := latest.ResourceStatus().ID

	// A Namespace ID is "<name>.<account ID>", and names can't contain dots.
	name, _, _ := strings.Cut(foundID, ".")
	if crName := latest.CR().(*tcloudv1alpha1.Namespace).Name; name != crName {
		return fmt.Errorf(
			"can't adopt tcloud namespace %q: its name %q doesn't match the CR's name %q",
			foundID, name, crName,
		)
	}
	return nil
}

package namespace

import (
	"strings"

	tcloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
	"github.com/temporalio/kube-temporal/pkg/kube/annotation"
	"github.com/temporalio/kube-temporal/pkg/types"
)

func adoptionPolicy(subject types.Resource) (tcloudcommon.AdoptionPolicy, error) {
	v, ok := annotation.Value(subject, tcloudcommon.AnnotationKeyAdoptionPolicy)

	if !ok {
		return "", nil
	}

	v = strings.ToLower(strings.TrimSpace(v))
	p := tcloudcommon.AdoptionPolicy(v)
	return p, p.Validate()
}

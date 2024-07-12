// SPDX-License-Identifier: AGPL-3.0-only
// Provenance-includes-location: https://github.com/kubernetes-sigs/apiserver-runtime/blob/main/pkg/experimental/storage/filepath/jsonfile_rest.go
// Provenance-includes-license: Apache-2.0
// Provenance-includes-copyright: The Kubernetes Authors.

package file

import (
	"bytes"
	"fmt"
	"strconv"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/storage"

	"github.com/grafana/grafana/pkg/storage/unified/resource"
)

func errorWrap(status *resource.StatusResult) error {
	if status != nil {
		return &apierrors.StatusError{ErrStatus: metav1.Status{
			Status:  metav1.StatusFailure,
			Code:    status.Code,
			Reason:  metav1.StatusReason(status.Reason),
			Message: status.Message,
		}}
	}
	return nil
}

func toListRequest(key string, opts storage.ListOptions) (*resource.ListRequest, storage.SelectionPredicate, error) {
	predicate := opts.Predicate
	k, err := getKey(key)
	if err != nil {
		return nil, predicate, err
	}
	req := &resource.ListRequest{
		Limit: opts.Predicate.Limit,
		Options: &resource.ListOptions{
			Key: k,
		},
		NextPageToken: predicate.Continue,
	}

	if opts.Predicate.Label != nil && !opts.Predicate.Label.Empty() {
		requirements, selectable := opts.Predicate.Label.Requirements()
		if !selectable {
			return nil, predicate, nil // not selectable
		}

		for _, r := range requirements {
			v := r.Key()

			req.Options.Labels = append(req.Options.Labels, &resource.Requirement{
				Key:      v,
				Operator: string(r.Operator()),
				Values:   r.Values().List(),
			})
		}
	}

	if opts.Predicate.Field != nil && !opts.Predicate.Field.Empty() {
		requirements := opts.Predicate.Field.Requirements()
		for _, r := range requirements {
			requirement := &resource.Requirement{Key: r.Field, Operator: string(r.Operator)}
			if r.Value != "" {
				requirement.Values = append(requirement.Values, r.Value)
			}
			req.Options.Labels = append(req.Options.Labels, requirement)
		}
	}

	if opts.ResourceVersion != "" {
		rv, err := strconv.ParseInt(opts.ResourceVersion, 10, 64)
		if err != nil {
			return nil, predicate, apierrors.NewBadRequest(fmt.Sprintf("invalid resource version: %s", opts.ResourceVersion))
		}
		req.ResourceVersion = rv
	}

	switch opts.ResourceVersionMatch {
	case "", metav1.ResourceVersionMatchNotOlderThan:
		req.VersionMatch = resource.ResourceVersionMatch_NotOlderThan
	case metav1.ResourceVersionMatchExact:
		req.VersionMatch = resource.ResourceVersionMatch_Exact
	default:
		return nil, predicate, apierrors.NewBadRequest(
			fmt.Sprintf("unsupported version match: %v", opts.ResourceVersionMatch),
		)
	}

	return req, predicate, nil
}

func isUnchanged(codec runtime.Codec, obj runtime.Object, newObj runtime.Object) (bool, error) {
	buf := new(bytes.Buffer)
	if err := codec.Encode(obj, buf); err != nil {
		return false, err
	}

	newBuf := new(bytes.Buffer)
	if err := codec.Encode(newObj, newBuf); err != nil {
		return false, err
	}

	return bytes.Equal(buf.Bytes(), newBuf.Bytes()), nil
}

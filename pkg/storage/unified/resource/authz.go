package resource

import (
	"context"

	"github.com/grafana/grafana/pkg/apimachinery/identity"
)

// Check if an individual item is viewable
type CanViewResource = func(key *ResourceKey) bool

// Throws an error (Forbidden) when unable to CRUD
// This is an internal abstraction and subject to change as the global authz solution evolves
type authzChecker interface {
	CanCreate(ctx context.Context, user identity.Requester, key *ResourceKey) error
	CanRead(ctx context.Context, user identity.Requester, key *ResourceKey) error
	CanUpdate(ctx context.Context, user identity.Requester, key *ResourceKey) error
	CanDelete(ctx context.Context, user identity.Requester, key *ResourceKey) error

	// When listing items in a namespace, get a callback function to see if an item is viewable
	CanList(ctx context.Context, user identity.Requester, key *ResourceKey) (CanViewResource, error)
}

var _ authzChecker = (*yesAuthzChecker)(nil)

type yesAuthzChecker struct{}

// CanCreate implements authzChecker.
func (y *yesAuthzChecker) CanCreate(ctx context.Context, user identity.Requester, key *ResourceKey) error {
	return nil
}

// CanDelete implements authzChecker.
func (y *yesAuthzChecker) CanDelete(ctx context.Context, user identity.Requester, key *ResourceKey) error {
	return nil
}

// CanList implements authzChecker.
func (y *yesAuthzChecker) CanList(ctx context.Context, user identity.Requester, key *ResourceKey) (CanViewResource, error) {
	return func(_ *ResourceKey) bool {
		return true
	}, nil
}

// CanRead implements authzChecker.
func (y *yesAuthzChecker) CanRead(ctx context.Context, user identity.Requester, key *ResourceKey) error {
	return nil
}

// CanUpdate implements authzChecker.
func (y *yesAuthzChecker) CanUpdate(ctx context.Context, user identity.Requester, key *ResourceKey) error {
	return nil
}

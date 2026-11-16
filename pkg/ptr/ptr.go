// Package ptr holds helpers for working with pointers.
package ptr

import "k8s.io/apimachinery/pkg/api/equality"

// NilIfEmpty returns nil if p points to a zero value, otherwise p.
func NilIfEmpty[T any](p *T) *T {
	var zero T
	if p != nil && equality.Semantic.DeepEqual(*p, zero) {
		return nil
	}
	return p
}

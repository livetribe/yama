// Package setother declares a provider that no stub file names. A provider set
// in another package states it.
package setother

type Thing struct{}

func NewThing() *Thing { return &Thing{} }

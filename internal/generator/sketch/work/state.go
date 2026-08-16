package work

import "context"

type State interface {
	PackagePath() (path string, ok bool)

	// Prepare is called before Google Wire is called.
	Prepare(ctx context.Context) State

	Generate(ctx context.Context) State

	// Complete is called to perform any cleanup duties after all the
	// lifecycle_gen.go files have been created.
	Complete(ctx context.Context) error
}

type Items []State

func (items Items) Paths() []string {
	var paths []string

	for _, item := range items {
		if path, ok := item.PackagePath(); ok {
			paths = append(paths, path)
		}
	}

	return paths
}

package work

import (
	"io"

	"l7e.io/yama/v2/internal/generator/sketch/custody"
	"l7e.io/yama/v2/internal/generator/sketch/source"
	"l7e.io/yama/v2/internal/generator/sketch/wire"
)

// CreateWorkItems returns one item for each target package. paths are the
// directories that hold them. prefix starts the name of every file a run
// settles. header holds the bytes that every lifecycle file carries above
// Yama's own provenance line, and it is nil when the run set no header file.
// tags are the build tags that the run set. progress is the stream that each
// item reports the file it wrote on.
//
// CreateWorkItems reads each package. The facts that it reads decide the first
// state of each package.
func CreateWorkItems(paths []string, prefix string, header []byte, tags []string, progress io.Writer) Items {
	items := make([]State, len(paths))

	for i, path := range paths {
		items[i] = createItem(path, prefix, header, tags, progress)
	}

	return items
}

// createItem returns the first state of the package in path. A package that
// the run cannot read starts as a CreateFailed. A package that declares no
// lifecycle stub starts as a NoStubs. Every other package starts as a Happy.
func createItem(path, prefix string, header []byte, tags []string, progress io.Writer) State {
	info, err := source.CollectPackageInfo(path, tags)
	if err != nil {
		return &CreateFailed{err: err}
	}

	custodian := custody.NewCustodian(path, prefix)
	intermediates := wire.NewIntermediateYamaFiles(info)

	if len(info.Stubs) == 0 {
		return &NoStubs{}
	}

	return NewHappy(path, custodian, intermediates, header, tags, info, progress)
}

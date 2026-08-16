package wire

import "l7e.io/yama/v2/internal/generator/sketch/source"

// An IntermediateYamaFiles is the pair of files that a run puts in a target
// package for Google Wire's load: the derived injector file and the
// placeholder file. The two go in together, and they come out together.
type IntermediateYamaFiles struct {
	info *source.PackageInfo
}

// NewIntermediateYamaFiles returns the pair for one target package. info states
// the directory that holds the pair, and the stubs that each file declares.
func NewIntermediateYamaFiles(info *source.PackageInfo) *IntermediateYamaFiles {
	return &IntermediateYamaFiles{info: info}
}

// Prepare puts both files in the package's directory.
func (iyf *IntermediateYamaFiles) Prepare() error {
	derived := Derive(iyf.info)

	if err := writeDerivedFile(iyf.info.Dir, derived); err != nil {
		return err
	}

	placeholder := DerivePlaceholder(iyf.info)

	return writePlaceholderFile(iyf.info.Dir, placeholder)
}

// CleanUp takes both files out of the package's directory. A directory that
// holds neither is not an error.
func (iyf *IntermediateYamaFiles) CleanUp() error {
	return removeDerivedFile(iyf.info.Dir)
}

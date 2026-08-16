package source

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ImportName", func() {
	It("takes the alias that the file gave", func() {
		imp := Import{Name: "applib", Path: "example.com/app/lib"}

		Expect(ImportName(imp, nil)).To(Equal("applib"))
	})

	It("takes the name that the package declares", func() {
		imp := Import{Path: "gopkg.in/yaml.v3"}
		names := map[string]string{"gopkg.in/yaml.v3": "yaml"}

		Expect(ImportName(imp, names)).To(Equal("yaml"))
	})

	It("takes the alias over the name that the package declares", func() {
		imp := Import{Name: "y", Path: "gopkg.in/yaml.v3"}
		names := map[string]string{"gopkg.in/yaml.v3": "yaml"}

		Expect(ImportName(imp, names)).To(Equal("y"))
	})

	It("guesses from the path when it has neither", func() {
		imp := Import{Path: "example.com/app/lib"}

		Expect(ImportName(imp, nil)).To(Equal("lib"))
	})
})

var _ = Describe("checkImports", func() {
	It("takes a package whose names each refer to one path", func() {
		imports := []Import{
			{Path: "context"},
			{Path: "example.com/app/lib"},
		}

		Expect(checkImports(imports, nil)).To(Succeed())
	})

	It("takes the same import stated by two files", func() {
		imports := []Import{
			{Path: "context"},
			{Path: "context"},
		}

		Expect(checkImports(imports, nil)).To(Succeed())
	})

	It("reports one name that two paths answer to", func() {
		imports := []Import{
			{Path: "example.com/a/lib"},
			{Path: "example.com/b/lib"},
		}

		err := checkImports(imports, nil)

		Expect(err).To(MatchError(ContainSubstring("the name lib refers to")))
		Expect(err).To(MatchError(ContainSubstring("example.com/a/lib")))
		Expect(err).To(MatchError(ContainSubstring("example.com/b/lib")))
	})

	It("reports a conflict that an alias created", func() {
		imports := []Import{
			{Path: "example.com/a/lib"},
			{Name: "lib", Path: "example.com/b/other"},
		}

		Expect(checkImports(imports, nil)).To(MatchError(ContainSubstring("the name lib refers to")))
	})

	It("takes a conflict that an alias settled", func() {
		imports := []Import{
			{Path: "example.com/a/lib"},
			{Name: "blib", Path: "example.com/b/lib"},
		}

		Expect(checkImports(imports, nil)).To(Succeed())
	})

	// Two paths that a guess reads as one name may declare names of their own
	// that differ. The name a package declares is what the check reads.
	It("reads the name that each package declares", func() {
		imports := []Import{
			{Path: "example.com/a/yaml.v3"},
			{Path: "example.com/b/yaml.v3"},
		}
		names := map[string]string{
			"example.com/a/yaml.v3": "yaml",
			"example.com/b/yaml.v3": "yamlv3",
		}

		Expect(checkImports(imports, names)).To(Succeed())
	})

	It("takes a package that imports nothing", func() {
		Expect(checkImports(nil, nil)).To(Succeed())
	})
})

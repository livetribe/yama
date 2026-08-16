// Copyright (c) 2026 the original author or authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package work

import (
	"context"
	"io"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"l7e.io/yama/v2/internal/generator/sketch/custody"
	"l7e.io/yama/v2/internal/generator/sketch/wire"
)

// A PIt in this file marks a spec pending. Ginkgo reports a pending spec
// separately from a pass.
//
// A scenario names the situation that a run meets. The comment above each one
// states what the target directory holds. Each scenario runs the three phases
// in order, and a nested context names a condition that arose during the phase
// before it.
//
// Each spec runs against a real directory, so a spec observes a move by reading
// the directory. A spec stands in for Google Wire: it writes the output that
// Google Wire would have written, between Prepare and Generate.

const (
	lifecycleName = "lifecycle_gen.go"
	wireName      = "wire_gen.go"
)

// componentsFile is the package that the target directory holds. NewDB returns
// a cleanup and declares no capability. App declares two.
//
// Boot calls the lifecycle constructor. The committed lifecycle file is set
// aside while Generate type-checks the package, so the package needs the
// placeholder that Prepare wrote to resolve that call.
const componentsFile = `package app

import "context"

func Boot(ctx context.Context) error {
	_, _, err := NewAppLifecycle(ctx)

	return err
}

type Logger struct{}

func NewLogger() *Logger { return &Logger{} }

type DB struct{}

func NewDB(l *Logger) (*DB, func()) { return &DB{}, func() {} }

type App struct{}

func NewApp(db *DB) *App { return &App{} }

func (a *App) Start(_ context.Context) error { return nil }
func (a *App) Stop(_ context.Context)        {}
`

// stubFile is the lifecycle stub that a person wrote.
const stubFile = `//go:build yamainject

package app

import (
	"context"

	"github.com/google/wire"

	yama "l7e.io/yama/v2"
)

// NewAppLifecycle builds the app and its lifecycle.
func NewAppLifecycle(ctx context.Context, opts ...yama.Option) (*App, yama.Lifecycle, error) {
	panic(wire.Build(NewLogger, NewDB, NewApp))
}
`

// emittableOutput is what Google Wire writes for the stub above. It parses and
// it type-checks, so Generate reaches the lifecycle file.
const emittableOutput = `//go:build !wireinject

package app

import "context"

func yama_NewAppLifecycle(ctx context.Context) (*App, func(), error) {
	logger := NewLogger()
	db, cleanup := NewDB(logger)
	app := NewApp(db)
	return app, func() { cleanup() }, nil
}
`

// unemittableOutput parses and does not type-check, so Generate fails at the
// step that reads what each component's type declares.
const unemittableOutput = `//go:build !wireinject

package app

import "context"

func yama_NewAppLifecycle(ctx context.Context) (*App, func(), error) {
	logger := NewMissing()
	app := NewApp(logger)
	return app, func() {}, nil
}
`

var _ = Describe("a work item over one target package", func() {
	var (
		dir  string
		item State
	)

	BeforeEach(func() {
		dir = GinkgoT().TempDir()
	})

	// CreateWorkItems reads the package. Each BeforeEach above puts the
	// directory in the state that its scenario names. This block runs after all
	// of them.
	JustBeforeEach(func() {
		items := CreateWorkItems([]string{dir}, "", nil, nil, io.Discard)
		item = items[0]
	})

	// write puts content at name in dir and fails the spec when it cannot.
	write := func(name, content string) {
		err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600)
		Expect(err).NotTo(HaveOccurred())
	}

	// writeTarget makes dir a module of its own and puts the package and its
	// stub in it. Generate type-checks the package, and the replace resolves the
	// stub's yama import against this repository.
	writeTarget := func() {
		repo, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
		Expect(err).NotTo(HaveOccurred())

		write("go.mod", "module app\n\ngo 1.25.0\n\nrequire l7e.io/yama/v2 v2.0.0\n\nreplace l7e.io/yama/v2 => "+repo+"\n")
		write("components.go", componentsFile)
		write("lifecycle.go", stubFile)
	}

	// denyWrites takes writes away from the package directory, so the custodian
	// cannot move either name aside. It gives the writes back afterwards, so a
	// later phase runs against a directory it can change.
	denyWrites := func() {
		Expect(os.Chmod(dir, 0o500)).To(Succeed())
		DeferCleanup(func() {
			_ = os.Chmod(dir, 0o700)
		})
	}

	// read returns what dir holds at name.
	read := func(name string) string {
		b, err := os.ReadFile(filepath.Join(dir, name))
		Expect(err).NotTo(HaveOccurred())

		return string(b)
	}

	// exists reports whether dir holds name.
	exists := func(name string) bool {
		_, err := os.Lstat(filepath.Join(dir, name))

		return err == nil
	}

	// backup returns the backup name for name.
	backup := func(name string) string {
		return custody.BackupPrefix + name
	}

	// prepared runs Prepare and returns what it produced.
	prepared := func() State {
		return item.Prepare(context.Background())
	}

	// generated runs Prepare and then Generate, and returns what Generate
	// produced. Google Wire wrote nothing between the two.
	generated := func() State {
		return prepared().Generate(context.Background())
	}

	// emitted runs Prepare, puts output that emits in the directory, and runs
	// Generate.
	emitted := func() State {
		state := prepared()
		write(wireName, emittableOutput)

		return state.Generate(context.Background())
	}

	// unemitted runs Prepare, puts output that fails to emit in the directory,
	// and runs Generate.
	unemitted := func() State {
		state := prepared()
		write(wireName, unemittableOutput)

		return state.Generate(context.Background())
	}

	// complete runs Complete on what a phase produced and drops what it
	// returned. A spec that reads the directory afterwards calls this one.
	complete := func(state State) {
		_ = state.Complete(context.Background())
	}

	// completed runs Complete on what a phase produced and returns what it
	// produced. A spec that asserts on the error calls this one.
	completed := func(state State) error {
		return state.Complete(context.Background())
	}

	// A clean first run. The directory holds neither generated file. Yama has
	// never generated for this package, and the package does not run Google Wire
	// for itself. Prepare moves nothing, so every later phase settles a directory
	// that holds no backup.
	Context("a clean first run", func() {
		BeforeEach(func() {
			writeTarget()
		})

		Describe("Prepare", func() {
			It("moves no file", func() {
				prepared()
				Expect(exists(backup(lifecycleName))).To(BeFalse())
				Expect(exists(backup(wireName))).To(BeFalse())
			})

		})

		Describe("Generate", func() {
			Context("and Google Wire wrote no output", func() {
				It("writes no lifecycle file", func() {
					generated()
					Expect(exists(lifecycleName)).To(BeFalse())
				})

				It("leaves both transient files where Prepare put them", func() {
					generated()
					Expect(exists(wire.DerivedFileName)).To(BeTrue())
					Expect(exists(wire.PlaceholderFileName)).To(BeTrue())
				})

				Describe("Complete", func() {
					It("finds no backup for either name", func() {
						complete(generated())
						Expect(exists(backup(lifecycleName))).To(BeFalse())
						Expect(exists(backup(wireName))).To(BeFalse())
					})

					It("leaves the directory as the run found it", func() {
						complete(generated())
						Expect(exists(lifecycleName)).To(BeFalse())
						Expect(exists(wireName)).To(BeFalse())
						Expect(exists(wire.DerivedFileName)).To(BeFalse())
						Expect(exists(wire.PlaceholderFileName)).To(BeFalse())
					})

					It("returns nil", func() {
						Expect(completed(generated())).To(Succeed())
					})
				})
			})

			Context("and Google Wire wrote output that emits", func() {
				It("writes lifecycle_gen.go", func() {
					emitted()
					Expect(read(lifecycleName)).To(ContainSubstring("rt.NewLifecycleBuilder(opts...)"))
				})

				It("removes wire_gen.go", func() {
					emitted()
					Expect(exists(wireName)).To(BeFalse())
				})

				It("removes both transient files", func() {
					emitted()
					Expect(exists(wire.DerivedFileName)).To(BeFalse())
					Expect(exists(wire.PlaceholderFileName)).To(BeFalse())
				})

				It("removes both transient files only after the write succeeds", func() {
					state := prepared()
					write(wireName, emittableOutput)
					denyWrites()

					settled := state.Generate(context.Background())

					Expect(exists(wireName)).To(BeTrue())
					Expect(exists(wire.DerivedFileName)).To(BeTrue())
					Expect(exists(wire.PlaceholderFileName)).To(BeTrue())
					Expect(completed(settled)).To(HaveOccurred())
				})

				Describe("Complete", func() {
					It("finds no backup for either name", func() {
						complete(emitted())
						Expect(exists(backup(lifecycleName))).To(BeFalse())
						Expect(exists(backup(wireName))).To(BeFalse())
					})

					It("leaves the emitted lifecycle_gen.go in the directory", func() {
						complete(emitted())
						Expect(read(lifecycleName)).To(ContainSubstring("Code generated by Yama"))
					})

					It("returns nil", func() {
						Expect(completed(emitted())).To(Succeed())
					})
				})
			})

			Context("and Google Wire wrote output that fails to emit", func() {
				It("writes no lifecycle_gen.go", func() {
					unemitted()
					Expect(exists(lifecycleName)).To(BeFalse())
				})

				It("leaves wire_gen.go for the rest of the loop", func() {
					unemitted()
					Expect(exists(wireName)).To(BeTrue())
				})

				It("leaves both transient files in the directory", func() {
					unemitted()
					Expect(exists(wire.DerivedFileName)).To(BeTrue())
					Expect(exists(wire.PlaceholderFileName)).To(BeTrue())
				})

				Describe("Complete", func() {
					It("removes wire_gen.go, which has no backup", func() {
						complete(unemitted())
						Expect(exists(wireName)).To(BeFalse())
					})

					It("returns the error that Generate held", func() {
						Expect(completed(unemitted())).To(HaveOccurred())
					})
				})
			})
		})
	})

	// A refresh. The directory holds a lifecycle_gen.go that an earlier run
	// committed, and no wire_gen.go, because that earlier run removed the one it
	// made. The committed file must move aside, or it declares the same
	// constructors as the placeholders in the derived injector file.
	Context("a refresh of a package Yama already generated", func() {
		BeforeEach(func() {
			writeTarget()
			write(lifecycleName, "committed\n")
		})

		Describe("Prepare", func() {
			It("moves lifecycle_gen.go to the backup name", func() {
				prepared()
				Expect(exists(backup(lifecycleName))).To(BeTrue())
			})

			It("leaves no file at the live lifecycle_gen.go name", func() {
				prepared()
				Expect(exists(lifecycleName)).To(BeFalse())
			})

		})

		Context("and SetAside could not take custody", func() {
			BeforeEach(func() {
				denyWrites()
			})

			Describe("Prepare", func() {
				It("keeps the package out of the Google Wire run", func() {
					_, ok := prepared().PackagePath()

					Expect(ok).To(BeFalse())
				})

				It("leaves the committed lifecycle_gen.go at its live name", func() {
					prepared()
					Expect(exists(lifecycleName)).To(BeTrue())
					Expect(exists(backup(lifecycleName))).To(BeFalse())
				})
			})

			Describe("Generate", func() {
				It("leaves the directory unchanged", func() {
					generated()
					Expect(read(lifecycleName)).To(Equal("committed\n"))
					Expect(exists(backup(lifecycleName))).To(BeFalse())
				})
			})

			Describe("Complete", func() {
				It("leaves lifecycle_gen.go where the run found it", func() {
					complete(generated())
					Expect(read(lifecycleName)).To(Equal("committed\n"))
				})

				It("returns the error that SetAside produced", func() {
					Expect(completed(generated())).To(HaveOccurred())
				})
			})
		})

		Context("and the item is still a Happy", func() {
			Describe("Generate", func() {
				Context("and Google Wire wrote output that emits", func() {
					It("writes a fresh lifecycle_gen.go over the live name", func() {
						emitted()
						Expect(read(lifecycleName)).To(ContainSubstring("Code generated by Yama"))
					})

					Describe("Complete", func() {
						It("deletes the lifecycle backup rather than restoring it", func() {
							complete(emitted())
							Expect(exists(backup(lifecycleName))).To(BeFalse())
						})

						It("leaves the fresh lifecycle_gen.go in the directory", func() {
							complete(emitted())
							Expect(read(lifecycleName)).To(ContainSubstring("Code generated by Yama"))
						})

						It("returns nil", func() {
							Expect(completed(emitted())).To(Succeed())
						})
					})
				})

				Context("and Google Wire wrote no output", func() {
					It("writes no lifecycle file", func() {
						generated()
						Expect(exists(lifecycleName)).To(BeFalse())
					})

					Describe("Complete", func() {
						It("restores the committed lifecycle_gen.go byte for byte", func() {
							complete(generated())
							Expect(read(lifecycleName)).To(Equal("committed\n"))
						})

						It("leaves no backup behind", func() {
							complete(generated())
							Expect(exists(backup(lifecycleName))).To(BeFalse())
						})

						It("returns nil", func() {
							Expect(completed(generated())).To(Succeed())
						})
					})
				})
			})
		})
	})

	// Adoption beside Google Wire. The application runs Google Wire for its own
	// injectors and commits the wire_gen.go that comes out. The directory holds
	// no lifecycle_gen.go, so this is the package's first Yama run. Yama borrows
	// the wire_gen.go name, so it must give the application's file back.
	Context("a package that also runs Google Wire for itself", func() {
		BeforeEach(func() {
			writeTarget()
			write(wireName, "legacy\n")
		})

		Describe("Prepare", func() {
			It("moves wire_gen.go to the backup name", func() {
				prepared()
				Expect(exists(backup(wireName))).To(BeTrue())
			})

		})

		Context("and SetAside could not take custody", func() {
			BeforeEach(func() {
				denyWrites()
			})

			Describe("Prepare", func() {
				It("keeps the package out of the Google Wire run", func() {
					_, ok := prepared().PackagePath()

					Expect(ok).To(BeFalse())
				})

				It("leaves the legacy wire_gen.go at its live name", func() {
					prepared()
					Expect(exists(wireName)).To(BeTrue())
					Expect(exists(backup(wireName))).To(BeFalse())
				})
			})

			Describe("Generate", func() {
				It("leaves the directory unchanged", func() {
					generated()
					Expect(read(wireName)).To(Equal("legacy\n"))
					Expect(exists(backup(wireName))).To(BeFalse())
				})
			})

			Describe("Complete", func() {
				// The custodian cannot tell a live wire_gen.go with no backup
				// from Google Wire's own output. It settles this one by the
				// record that SetAside left, and it must not delete a file the
				// application owns and Yama cannot write again.
				It("does not delete the file the application owns", func() {
					settled := prepared()

					Expect(settled.Complete(context.Background())).NotTo(Succeed())
					Expect(read(wireName)).To(Equal("legacy\n"))
				})

				It("returns the error that SetAside produced", func() {
					Expect(completed(generated())).To(HaveOccurred())
				})
			})
		})

		Context("and the item is still a Happy", func() {
			Describe("Generate", func() {
				Context("and Google Wire wrote output that emits", func() {
					It("writes lifecycle_gen.go", func() {
						emitted()
						Expect(read(lifecycleName)).To(ContainSubstring("Code generated by Yama"))
					})

					It("removes the wire_gen.go that Google Wire wrote", func() {
						emitted()
						Expect(exists(wireName)).To(BeFalse())
					})

					Describe("Complete", func() {
						It("restores the legacy wire_gen.go byte for byte", func() {
							complete(emitted())
							Expect(read(wireName)).To(Equal("legacy\n"))
						})

						It("returns nil", func() {
							Expect(completed(emitted())).To(Succeed())
						})
					})
				})

				Context("and Google Wire wrote no output", func() {
					Describe("Complete", func() {
						It("restores the legacy wire_gen.go byte for byte", func() {
							complete(generated())
							Expect(read(wireName)).To(Equal("legacy\n"))
						})

						It("returns nil", func() {
							Expect(completed(generated())).To(Succeed())
						})
					})
				})
			})
		})
	})

	// A refresh beside Google Wire. The directory holds both files. They settle
	// on different terms: Yama writes the lifecycle file again, and Yama can
	// never write the application's wire_gen.go again. This is the scenario that
	// the move order of Rule 2 exists for.
	Context("a refresh of a package that also runs Google Wire for itself", func() {
		BeforeEach(func() {
			writeTarget()
			write(lifecycleName, "committed\n")
			write(wireName, "legacy\n")
		})

		Describe("Prepare", func() {
			It("moves both names to their backup names", func() {
				prepared()
				Expect(exists(backup(lifecycleName))).To(BeTrue())
				Expect(exists(backup(wireName))).To(BeTrue())
			})

		})

		Context("and SetAside could not take custody", func() {
			BeforeEach(func() {
				denyWrites()
			})

			Describe("Prepare", func() {
				It("keeps the package out of the Google Wire run", func() {
					_, ok := prepared().PackagePath()

					Expect(ok).To(BeFalse())
				})

				It("leaves both files at their live names", func() {
					prepared()
					Expect(exists(lifecycleName)).To(BeTrue())
					Expect(exists(wireName)).To(BeTrue())
				})

				It("creates no backup for either name", func() {
					prepared()
					Expect(exists(backup(lifecycleName))).To(BeFalse())
					Expect(exists(backup(wireName))).To(BeFalse())
				})
			})

			Describe("Generate", func() {
				It("leaves the directory unchanged", func() {
					generated()
					Expect(read(lifecycleName)).To(Equal("committed\n"))
					Expect(read(wireName)).To(Equal("legacy\n"))
				})
			})

			Describe("Complete", func() {
				It("leaves both files where the run found them", func() {
					complete(generated())
					Expect(read(lifecycleName)).To(Equal("committed\n"))
					Expect(read(wireName)).To(Equal("legacy\n"))
				})

				It("returns the error that SetAside produced", func() {
					Expect(completed(generated())).To(HaveOccurred())
				})
			})
		})

		Context("and the item is still a Happy", func() {
			Describe("Generate", func() {
				Context("and Google Wire wrote output that emits", func() {
					It("writes a fresh lifecycle_gen.go", func() {
						emitted()
						Expect(read(lifecycleName)).To(ContainSubstring("Code generated by Yama"))
					})

					Describe("Complete", func() {
						It("deletes the lifecycle backup rather than restoring it", func() {
							complete(emitted())
							Expect(exists(backup(lifecycleName))).To(BeFalse())
							Expect(read(lifecycleName)).To(ContainSubstring("Code generated by Yama"))
						})

						It("restores the legacy wire_gen.go byte for byte", func() {
							complete(emitted())
							Expect(read(wireName)).To(Equal("legacy\n"))
						})

						It("returns nil", func() {
							Expect(completed(emitted())).To(Succeed())
						})
					})
				})

				Context("and Google Wire wrote no output", func() {
					Describe("Complete", func() {
						It("restores the committed lifecycle_gen.go byte for byte", func() {
							complete(generated())
							Expect(read(lifecycleName)).To(Equal("committed\n"))
						})

						It("restores the legacy wire_gen.go byte for byte", func() {
							complete(generated())
							Expect(read(wireName)).To(Equal("legacy\n"))
						})

						It("leaves the package exactly as the run found it", func() {
							complete(generated())
							Expect(exists(backup(lifecycleName))).To(BeFalse())
							Expect(exists(backup(wireName))).To(BeFalse())
							Expect(exists(wire.DerivedFileName)).To(BeFalse())
							Expect(exists(wire.PlaceholderFileName)).To(BeFalse())
						})

						It("returns nil", func() {
							Expect(completed(generated())).To(Succeed())
						})
					})
				})

				Context("and Google Wire wrote output that fails to emit", func() {
					It("leaves wire_gen.go for the rest of the loop", func() {
						unemitted()
						Expect(exists(wireName)).To(BeTrue())
					})

					Describe("Complete", func() {
						It("restores both names in its record", func() {
							complete(unemitted())
							Expect(read(lifecycleName)).To(Equal("committed\n"))
							Expect(read(wireName)).To(Equal("legacy\n"))
						})

						It("returns the error that Generate held", func() {
							Expect(completed(unemitted())).To(HaveOccurred())
						})
					})
				})
			})
		})
	})

	// A package whose stubs do not load. The directory holds a wire_gen.go that
	// the application owns, and a stub file that does not parse. The run reads
	// the package before it takes custody of any directory. The failed load
	// leaves every name where the run found it.
	Context("a package whose stubs do not load", func() {
		BeforeEach(func() {
			write(wireName, "legacy\n")
			write("broken_yamainject.go", "//go:build yamainject\n\npackage app\n\nfunc (\n")
		})

		It("states no path, and Google Wire runs over no such package", func() {
			_, ok := item.PackagePath()
			Expect(ok).To(BeFalse())
		})

		Describe("Prepare", func() {
			It("leaves the application's wire_gen.go at its live name", func() {
				prepared()
				Expect(read(wireName)).To(Equal("legacy\n"))
				Expect(exists(backup(wireName))).To(BeFalse())
			})

			Describe("Complete", func() {
				It("leaves the application's wire_gen.go byte for byte", func() {
					complete(prepared())
					Expect(read(wireName)).To(Equal("legacy\n"))
				})

				It("leaves no backup behind", func() {
					complete(prepared())
					Expect(exists(backup(wireName))).To(BeFalse())
				})

				It("returns the error that the load produced", func() {
					Expect(completed(prepared())).To(HaveOccurred())
				})
			})
		})
	})

	// A package that declares no stub and runs Google Wire for itself. Yama has
	// no constructor to write here. The run states no path for the package, so
	// Google Wire never writes over the application's committed output.
	Context("a package that declares no stub and runs Google Wire for itself", func() {
		BeforeEach(func() {
			write(wireName, "legacy\n")
		})

		It("states no path, so Google Wire does not run over the package", func() {
			path, ok := item.PackagePath()

			Expect(ok).To(BeFalse())
			Expect(path).To(BeEmpty())
		})

		Describe("Prepare", func() {
			It("leaves the application's wire_gen.go at the live name", func() {
				prepared()
				Expect(exists(backup(wireName))).To(BeFalse())
				Expect(read(wireName)).To(Equal("legacy\n"))
			})

			It("writes neither transient file", func() {
				prepared()
				Expect(exists(wire.DerivedFileName)).To(BeFalse())
				Expect(exists(wire.PlaceholderFileName)).To(BeFalse())
			})
		})

		Describe("Generate", func() {
			It("writes no lifecycle file", func() {
				generated()

				Expect(exists(lifecycleName)).To(BeFalse())
			})

			Describe("Complete", func() {
				It("hands the application's own wire_gen.go back untouched", func() {
					settled := generated()

					Expect(completed(settled)).To(Succeed())
					Expect(read(wireName)).To(Equal("legacy\n"))
					Expect(exists(backup(wireName))).To(BeFalse())
				})
			})
		})
	})
})

var _ = Describe("CreateWorkItems", func() {
	// stubbed returns the directory of a package that declares one lifecycle
	// stub. CreateWorkItems reads each directory that it receives. Every target
	// below is a directory that holds a package.
	stubbed := func() string {
		dir := GinkgoT().TempDir()

		err := os.WriteFile(filepath.Join(dir, "lifecycle.go"), []byte(stubFile), 0o600)
		Expect(err).NotTo(HaveOccurred())

		return dir
	}

	// targets returns the directories of two packages that each declare one
	// lifecycle stub.
	targets := func() []string {
		return []string{stubbed(), stubbed()}
	}

	It("gives each item the header that every lifecycle file carries", func() {
		items := CreateWorkItems(targets(), "", []byte("// Copyright.\n"), nil, io.Discard)

		for _, item := range items {
			happy, ok := item.(*Happy)

			Expect(ok).To(BeTrue())
			Expect(string(happy.header)).To(Equal("// Copyright.\n"))
		}
	})

	It("gives each item no header when the run named no header file", func() {
		items := CreateWorkItems([]string{stubbed()}, "", nil, nil, io.Discard)

		Expect(items[0].(*Happy).header).To(BeNil())
	})

	It("gives each item the directory of its own package", func() {
		paths := targets()

		items := CreateWorkItems(paths, "", nil, nil, io.Discard)

		for i, item := range items {
			happy, ok := item.(*Happy)

			Expect(ok).To(BeTrue())
			Expect(happy.path).To(Equal(paths[i]))
		}
	})

	// The facts that CreateWorkItems reads decide the first state of each
	// package. CreateWorkItems reads each package into one PackageInfo. A Happy
	// carries that record through every later phase.
	It("gives each item the stubs that its own package declares", func() {
		items := CreateWorkItems([]string{stubbed()}, "", nil, nil, io.Discard)

		stubs := items[0].(*Happy).info.Stubs

		Expect(stubs).To(HaveLen(1))
		Expect(stubs[0].Name).To(Equal("NewAppLifecycle"))
	})

	It("carries the error of a directory it cannot read to Complete", func() {
		missing := filepath.Join(GinkgoT().TempDir(), "absent")

		items := CreateWorkItems([]string{missing}, "", nil, nil, io.Discard)

		Expect(items[0].Complete(context.Background())).To(HaveOccurred())
	})
})

// Each spec below states a question that neither the sketch nor a document
// answers.

var _ = Describe("a work item, where the design is undecided", func() {
	Context("when the directory already holds a backup name", func() {
		PIt("states what Prepare does with the backup that an earlier run left", func() {})
	})

	Context("when a restore inside Complete fails", func() {
		PIt("states what Complete returns", func() {})
	})

	Context("when Generate converts a Happy to a NoWireGen", func() {
		PIt("states whether NoWireGen carries the record unchanged", func() {})
	})
})

// The sketch is the design of record. Each spec below states behavior that
// ADR-014 or docs/generator-architecture.md describes and that no method of
// the sketch performs.

var _ = Describe("a work item, where the documents and the sketch disagree", func() {
	Describe("Prepare", func() {
		PIt("returns a PrepareFailed when the derivation fails", func() {})
	})

	Describe("Complete", func() {
		PIt("joins the errors that its restore calls produced", func() {})
	})

	Describe("the progress line", func() {
		PIt("writes one line to the run's stream when it commits a file", func() {})
	})
})

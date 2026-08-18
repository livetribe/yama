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
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"l7e.io/yama/v2/internal/generator/custody"
	"l7e.io/yama/v2/internal/generator/wire"
)

// A PIt in this file marks a spec pending. Ginkgo reports a pending spec
// separately from a pass.
//
// A scenario names the situation that a run meets. The comment above each
// scenario states what the target directory holds. Each scenario runs the three
// phases in order. A nested context names a condition that arose during the
// phase before it.
//
// Each spec runs against a real directory, so a spec observes a move by reading
// the directory. A spec takes the place of Google Wire. It writes the output
// that Google Wire would have written, between Prepare and Generate.

const (
	lifecycleName = "lifecycle_gen.go"
	wireName      = "wire_gen.go"
)

// fixtureRoot holds the files that a spec puts in the directory that it runs
// against. testdata/target holds the target package and its stub.
// testdata/output holds one directory for each Google Wire output, and the
// comment at the top of each one states what that output does to Generate.
const fixtureRoot = "testdata"

// targetFixture names the directory that holds the target package.
const targetFixture = "target"

// outputRoot holds one directory for each Google Wire output. Each constant
// below names one of them.
const outputRoot = "output"

const (
	emittableFixture   = "emittable"
	unemittableFixture = "unemittable"
	collidingFixture   = "colliding"
	unparsableFixture  = "unparsable"
)

// fixture returns what testdata holds at name.
func fixture(name string) string {
	path := filepath.Join(fixtureRoot, name)

	b, err := os.ReadFile(path)
	Expect(err).NotTo(HaveOccurred())

	return string(b)
}

var _ = Describe("a work item over one target package", func() {
	var (
		dir  string
		item State

		// progress holds what the run wrote to its stream.
		progress *bytes.Buffer
	)

	BeforeEach(func() {
		dir = GinkgoT().TempDir()
		progress = new(bytes.Buffer)
	})

	// CreateWorkItems reads the package. Each BeforeEach above puts the
	// directory in the state that its scenario names. This block runs after all
	// of them.
	JustBeforeEach(func() {
		items := CreateWorkItems([]string{dir}, "", nil, nil, progress)
		item = items[0]
	})

	// write puts content at name in dir and fails the spec when it cannot.
	write := func(name, content string) {
		err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600)
		Expect(err).NotTo(HaveOccurred())
	}

	// writeTarget makes dir a module of its own and puts the package and its
	// stub in it. Generate type-checks the package, and the replace resolves the
	// stub's yama import against this repository. The go.mod write comes last,
	// so dir holds this go.mod and not one that the fixture directory holds.
	writeTarget := func() {
		entries, err := os.ReadDir(filepath.Join(fixtureRoot, targetFixture))
		Expect(err).NotTo(HaveOccurred())

		for _, entry := range entries {
			content := fixture(filepath.Join(targetFixture, entry.Name()))
			write(entry.Name(), content)
		}

		repo, err := filepath.Abs(filepath.Join("..", "..", ".."))
		Expect(err).NotTo(HaveOccurred())

		write("go.mod", "module app\n\ngo 1.25.0\n\nrequire l7e.io/yama/v2 v2.0.0\n\nreplace l7e.io/yama/v2 => "+repo+"\n")
	}

	// writeOutput puts the named fixture at the wire_gen.go name in dir. A spec
	// calls this one between Prepare and Generate, where Google Wire would have
	// written its own output.
	writeOutput := func(name string) {
		content := fixture(filepath.Join(outputRoot, name, wireName))
		write(wireName, content)
	}

	// denyWrites removes write permission from the package directory, so a
	// phase that writes in it fails. It gives the permission back afterwards,
	// so a later phase runs against a directory that it can change.
	//
	// Windows does not apply the permission bits of a directory, and the
	// superuser writes to a read-only directory, so a spec that needs the
	// failure stops on both.
	denyWrites := func() {
		if runtime.GOOS == "windows" {
			Skip("Windows does not apply the permission bits of a directory, so the failure cannot be staged")
		}

		if os.Geteuid() == 0 {
			Skip("the superuser writes to a read-only directory, so the failure cannot be staged")
		}

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
		return item.Prepare()
	}

	// generated runs Prepare and then Generate, and returns what Generate
	// produced. Google Wire wrote nothing between the two.
	generated := func() State {
		return prepared().Generate()
	}

	// emitted runs Prepare, puts output that emits in the directory, and runs
	// Generate.
	emitted := func() State {
		state := prepared()
		writeOutput(emittableFixture)

		return state.Generate()
	}

	// unemitted runs Prepare, puts output that fails to emit in the directory,
	// and runs Generate.
	unemitted := func() State {
		state := prepared()
		writeOutput(unemittableFixture)

		return state.Generate()
	}

	// collided runs Prepare, puts output with an import block that collides
	// with the stub file's block in the directory, and runs Generate.
	collided := func() State {
		state := prepared()
		writeOutput(collidingFixture)

		return state.Generate()
	}

	// unparsed runs Prepare, puts output that states no ordering in the
	// directory, and runs Generate.
	unparsed := func() State {
		state := prepared()
		writeOutput(unparsableFixture)

		return state.Generate()
	}

	// complete runs Complete on what a phase produced, and it drops what
	// Complete returned. A spec that reads the directory afterwards calls this
	// one.
	complete := func(state State) {
		_ = state.Complete()
	}

	// completed runs Complete on what a phase produced and returns what it
	// produced. A spec that asserts on the error calls this one.
	completed := func(state State) error {
		return state.Complete()
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

				It("writes no progress line", func() {
					generated()
					Expect(progress.String()).To(BeEmpty())
				})

				It("settles as a package that Google Wire wrote nothing for", func() {
					Expect(generated()).To(BeAssignableToTypeOf(&NoWireGen{}))
				})

				It("keeps the package out of a later Google Wire run", func() {
					_, runWire := generated().PackagePath()

					Expect(runWire).To(BeFalse())
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

				It("writes one progress line naming the file it wrote", func() {
					emitted()

					line := fmt.Sprintf("yama: app: wrote %s\n", filepath.Join(dir, lifecycleName))

					Expect(progress.String()).To(Equal(line))
				})

				It("removes both transient files only after the write succeeds", func() {
					state := prepared()
					writeOutput(emittableFixture)
					denyWrites()

					settled := state.Generate()

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

				It("keeps the package out of a later Google Wire run", func() {
					_, runWire := unemitted().PackagePath()

					Expect(runWire).To(BeFalse())
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

					It("names the package on that error", func() {
						Expect(completed(unemitted())).To(MatchError(ContainSubstring(dir)))
					})
				})
			})

			// The lifecycle file carries the stub file's import block and Google
			// Wire's own. One name in that file refers to one path.
			Context("and Google Wire wrote output with an import block that collides", func() {
				It("settles as a package that failed to generate", func() {
					Expect(collided()).To(BeAssignableToTypeOf(&GenerateFailed{}))
				})

				It("writes no lifecycle_gen.go", func() {
					collided()
					Expect(exists(lifecycleName)).To(BeFalse())
				})

				Describe("Complete", func() {
					It("returns the error that Generate held", func() {
						Expect(completed(collided())).To(MatchError(ContainSubstring("needs an alias")))
					})
				})
			})

			// A statement that states no ordering builds a value that would reach
			// no lifecycle level.
			Context("and Google Wire wrote output that states no ordering", func() {
				It("settles as a package that failed to generate", func() {
					Expect(unparsed()).To(BeAssignableToTypeOf(&GenerateFailed{}))
				})

				It("writes no lifecycle_gen.go", func() {
					unparsed()
					Expect(exists(lifecycleName)).To(BeFalse())
				})

				Describe("Complete", func() {
					It("returns the error that Generate held", func() {
						Expect(completed(unparsed())).To(MatchError(ContainSubstring("unsupported statement")))
					})
				})
			})
		})
	})

	// A refresh. The directory holds a lifecycle_gen.go that an earlier run
	// committed, and no wire_gen.go, because that earlier run removed the file
	// that it made. The committed file must move aside, or it declares the same
	// constructors as the placeholders in the derived injector file.
	Context("a refresh of a package that Yama already generated", func() {
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

		Context("and Prepare could not write in the directory", func() {
			BeforeEach(func() {
				denyWrites()
			})

			Describe("Prepare", func() {
				It("keeps the package out of the Google Wire run", func() {
					_, runWire := prepared().PackagePath()

					Expect(runWire).To(BeFalse())
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

				It("returns the error that Prepare produced", func() {
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
	// injectors and commits the wire_gen.go that it produced. The directory holds
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

		Context("and Prepare could not write in the directory", func() {
			BeforeEach(func() {
				denyWrites()
			})

			Describe("Prepare", func() {
				It("keeps the package out of the Google Wire run", func() {
					_, runWire := prepared().PackagePath()

					Expect(runWire).To(BeFalse())
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
				// The custodian cannot separate a live wire_gen.go with no
				// backup from Google Wire's own output. It settles this file by
				// the record that SetAside left. It must not delete a file that
				// the application owns and that Yama cannot write again.
				It("does not delete the file that the application owns", func() {
					settled := prepared()

					Expect(settled.Complete()).NotTo(Succeed())
					Expect(read(wireName)).To(Equal("legacy\n"))
				})

				It("returns the error that Prepare produced", func() {
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

						// This state holds no error of its own, so the restore
						// is the only call that can produce one. The error
						// names the backup that still holds the file that
						// the application owns.
						It("returns the error of a restore that it could not make", func() {
							settled := generated()

							denyWrites()

							err := settled.Complete()

							Expect(err).To(MatchError(ContainSubstring("restore " + wireName)))
							Expect(err).To(MatchError(ContainSubstring(backup(wireName))))
						})
					})
				})
			})
		})
	})

	// A refresh beside Google Wire. The directory holds both files. They settle
	// on different terms. Yama writes the lifecycle file again, and Yama can
	// never write the application's wire_gen.go again. The move order of Rule 2
	// exists for this scenario.
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

		Context("and Prepare could not write in the directory", func() {
			BeforeEach(func() {
				denyWrites()
			})

			Describe("Prepare", func() {
				It("keeps the package out of the Google Wire run", func() {
					_, runWire := prepared().PackagePath()

					Expect(runWire).To(BeFalse())
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

				It("returns the error that Prepare produced", func() {
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

	// A refresh of a package. Its directory already holds one of the two
	// intermediate names. Prepare writes the intermediate files before it moves
	// the committed files aside, so the name that it cannot take stops it at
	// the first step. The committed lifecycle file stays where the run found
	// it, and every package that imports this one still declares what that file
	// declares for the rest of the run.
	Context("a refresh of a package that already holds an intermediate name", func() {
		BeforeEach(func() {
			writeTarget()
			write(lifecycleName, "committed\n")
			write(wire.DerivedFileName, "package app\n")
		})

		Describe("Prepare", func() {
			It("settles as a package that failed to prepare", func() {
				Expect(prepared()).To(BeAssignableToTypeOf(&PrepareFailed{}))
			})

			It("leaves the committed lifecycle_gen.go at its own name", func() {
				prepared()
				Expect(read(lifecycleName)).To(Equal("committed\n"))
			})

			It("moves nothing to a backup name", func() {
				prepared()
				Expect(exists(backup(lifecycleName))).To(BeFalse())
			})
		})

		Describe("Complete", func() {
			It("returns the error that Prepare held", func() {
				Expect(completed(prepared())).To(MatchError(ContainSubstring("already holds that name")))
			})

			It("leaves the file the user owns at the intermediate name", func() {
				complete(prepared())
				Expect(read(wire.DerivedFileName)).To(Equal("package app\n"))
			})
		})
	})

	// A package with a live lifecycle name that the run cannot take. A run that
	// did not finish left a backup, so the custodian deletes the live name
	// rather than move it. No remove takes out a directory that holds a file.
	//
	// Prepare writes the intermediate files before it asks for custody, so this
	// directory reaches the custody step and stops there.
	Context("a package with a live name that the run cannot take", func() {
		BeforeEach(func() {
			writeTarget()
			write(backup(lifecycleName), "committed\n")

			held := filepath.Join(dir, lifecycleName)
			Expect(os.Mkdir(held, 0o700)).To(Succeed())
			write(filepath.Join(lifecycleName, "held.txt"), "held\n")
		})

		Describe("Prepare", func() {
			It("settles as a package that failed to prepare", func() {
				Expect(prepared()).To(BeAssignableToTypeOf(&PrepareFailed{}))
			})

			It("wrote both intermediate files before it asked for custody", func() {
				prepared()
				Expect(exists(wire.DerivedFileName)).To(BeTrue())
				Expect(exists(wire.PlaceholderFileName)).To(BeTrue())
			})
		})

		Describe("Complete", func() {
			It("returns the error that custody produced", func() {
				Expect(completed(prepared())).To(MatchError(ContainSubstring("set aside")))
			})

			It("takes both intermediate files back out", func() {
				complete(prepared())
				Expect(exists(wire.DerivedFileName)).To(BeFalse())
				Expect(exists(wire.PlaceholderFileName)).To(BeFalse())
			})
		})
	})

	// A package with stubs that do not load. The directory holds a wire_gen.go
	// that the application owns, and a stub file that does not parse. The run
	// reads the package before it takes custody of any directory. The failed
	// load keeps every name where the run found it.
	Context("a package with stubs that do not load", func() {
		BeforeEach(func() {
			write(wireName, "legacy\n")
			write("broken_yamainject.go", "//go:build yamainject\n\npackage app\n\nfunc (\n")
		})

		It("states no path, and Google Wire runs over no such package", func() {
			_, runWire := item.PackagePath()
			Expect(runWire).To(BeFalse())
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
			path, runWire := item.PackagePath()

			Expect(runWire).To(BeFalse())
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
		stub := fixture(filepath.Join(targetFixture, "lifecycle.go"))

		err := os.WriteFile(filepath.Join(dir, "lifecycle.go"), []byte(stub), 0o600)
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
	// package. CreateWorkItems reads each package into one Info. A Happy carries
	// that record through every later phase.
	It("gives each item the stubs that its own package declares", func() {
		items := CreateWorkItems([]string{stubbed()}, "", nil, nil, io.Discard)

		stubs := items[0].(*Happy).info.Stubs()

		Expect(stubs).To(HaveLen(1))
		Expect(stubs[0].Name).To(Equal("NewAppLifecycle"))
	})

	It("carries the error of a directory that it cannot read to Complete", func() {
		missing := filepath.Join(GinkgoT().TempDir(), "absent")

		items := CreateWorkItems([]string{missing}, "", nil, nil, io.Discard)

		Expect(items[0].Complete()).To(HaveOccurred())
	})
})

// The spec below states a question that neither the code nor a document
// answers.

var _ = Describe("a work item, where the design is undecided", func() {
	PIt("states whether the NoWireGen that Generate returns carries the record unchanged", func() {})
})

// The code is the design of record. The spec below states behavior that
// ADR-014 or docs/generator-architecture.md describes and that no method of
// the code performs.

var _ = Describe("a work item, where the documents and the code disagree", func() {
	PIt("returns a PrepareFailed when the derivation fails", func() {})
})

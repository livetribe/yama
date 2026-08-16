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

package custody_test

import (
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"l7e.io/yama/v2/internal/generator/sketch/custody"
)

// The two tables in custody.md drive the contexts below, and each context names
// the row it covers. A Custodian settles both names on every call, so a context
// that sets up one name leaves the other absent, where its own rows do nothing.
const (
	lifecycle = "lifecycle_gen.go"
	wire      = "wire_gen.go"
)

var _ = Describe("Custodian", func() {
	var (
		dir string
		c   *custody.Custodian
	)

	BeforeEach(func() {
		dir = GinkgoT().TempDir()
		c = custody.NewCustodian(dir, "")
	})

	// write puts content at name in dir and fails the spec when it cannot.
	write := func(name, content string) {
		err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600)
		Expect(err).NotTo(HaveOccurred())
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

	// The set-aside table treats both names the same way, so one block of
	// specs runs over each. Rows 1 to 4 cover the lifecycle file and rows 5 to
	// 8 cover Google Wire's output.
	Describe("SetAside", func() {
		for _, owned := range []struct {
			name     string
			owner    string
			content  string
			firstRow int
		}{
			{lifecycle, "the lifecycle file, which Yama owns", "committed\n", 1},
			{wire, "Google Wire's output, which the application owns", "application\n", 5},
		} {
			row := func(n int) string {
				return fmt.Sprintf("(row %d)", owned.firstRow+n)
			}

			Context(owned.owner, func() {
				Context("when the directory holds neither name "+row(0), func() {
					It("moves nothing", func() {
						Expect(c.SetAside()).To(Succeed())
						Expect(exists(owned.name)).To(BeFalse())
						Expect(exists(backup(owned.name))).To(BeFalse())
					})
				})

				Context("when a run that did not finish left a backup "+row(1), func() {
					BeforeEach(func() {
						write(backup(owned.name), owned.content)
					})

					It("leaves that backup alone", func() {
						Expect(c.SetAside()).To(Succeed())
						Expect(read(backup(owned.name))).To(Equal(owned.content))
					})
				})

				Context("when the owner committed the file "+row(2), func() {
					BeforeEach(func() {
						write(owned.name, owned.content)
					})

					It("moves the content to the backup name", func() {
						Expect(c.SetAside()).To(Succeed())
						Expect(read(backup(owned.name))).To(Equal(owned.content))
					})

					It("leaves no file at the live name", func() {
						Expect(c.SetAside()).To(Succeed())
						Expect(exists(owned.name)).To(BeFalse())
					})
				})

				Context("when the file and a backup are both present "+row(3), func() {
					BeforeEach(func() {
						write(owned.name, "from the dead run\n")
						write(backup(owned.name), owned.content)
					})

					It("keeps the backup, which holds the file the owner committed", func() {
						Expect(c.SetAside()).To(Succeed())
						Expect(read(backup(owned.name))).To(Equal(owned.content))
					})

					It("deletes what the dead run left at the live name", func() {
						Expect(c.SetAside()).To(Succeed())
						Expect(exists(owned.name)).To(BeFalse())
					})
				})
			})
		}
	})

	Describe("Complete", func() {
		Context("the lifecycle file, which Yama owns", func() {
			Context("when the directory holds neither name (row 9)", func() {
				It("leaves the directory as it found it", func() {
					c.Complete()
					Expect(exists(lifecycle)).To(BeFalse())
					Expect(exists(backup(lifecycle))).To(BeFalse())
				})
			})

			Context("when the run emitted nothing (row 10)", func() {
				BeforeEach(func() {
					write(backup(lifecycle), "committed\n")
				})

				It("returns the committed file byte for byte", func() {
					c.Complete()
					Expect(read(lifecycle)).To(Equal("committed\n"))
				})

				It("leaves no file at the backup name", func() {
					c.Complete()
					Expect(exists(backup(lifecycle))).To(BeFalse())
				})
			})

			Context("when the run emitted a file over an empty name (row 11)", func() {
				BeforeEach(func() {
					write(lifecycle, "emitted\n")
				})

				It("keeps what the run emitted", func() {
					c.Complete()
					Expect(read(lifecycle)).To(Equal("emitted\n"))
				})
			})

			Context("when the run emitted a file over a committed one (row 12)", func() {
				BeforeEach(func() {
					write(lifecycle, "emitted\n")
					write(backup(lifecycle), "committed\n")
				})

				It("keeps what the run emitted", func() {
					c.Complete()
					Expect(read(lifecycle)).To(Equal("emitted\n"))
				})

				It("deletes the backup, because the emitted file replaced it", func() {
					c.Complete()
					Expect(exists(backup(lifecycle))).To(BeFalse())
				})
			})
		})

		Context("Google Wire's output, which the application owns", func() {
			Context("when the directory holds neither name (row 13)", func() {
				It("leaves the directory as it found it", func() {
					c.Complete()
					Expect(exists(wire)).To(BeFalse())
					Expect(exists(backup(wire))).To(BeFalse())
				})
			})

			Context("when Google Wire wrote nothing this run (row 14)", func() {
				BeforeEach(func() {
					write(backup(wire), "application\n")
				})

				It("returns the application's file byte for byte", func() {
					c.Complete()
					Expect(read(wire)).To(Equal("application\n"))
				})
			})

			Context("when Google Wire wrote over an empty name (row 15)", func() {
				BeforeEach(func() {
					write(wire, "generated\n")
				})

				It("deletes what Google Wire wrote", func() {
					c.Complete()
					Expect(exists(wire)).To(BeFalse())
				})
			})

			Context("when Google Wire wrote over the application's file (row 16)", func() {
				BeforeEach(func() {
					write(wire, "generated\n")
					write(backup(wire), "application\n")
				})

				It("returns the application's file byte for byte", func() {
					c.Complete()
					Expect(read(wire)).To(Equal("application\n"))
				})

				It("leaves no file at the backup name", func() {
					c.Complete()
					Expect(exists(backup(wire))).To(BeFalse())
				})
			})

			Context("when SetAside could not take custody (row 15)", func() {
				BeforeEach(func() {
					write(wire, "application\n")

					// A directory that denies writes makes the move fail, and
					// leaves the application's file at the live name with no
					// backup beside it.
					Expect(os.Chmod(dir, 0o500)).To(Succeed())
					DeferCleanup(func() {
						_ = os.Chmod(dir, 0o700)
					})

					Expect(c.SetAside()).NotTo(Succeed())

					// Give the directory its writes back, so a surviving file
					// proves the record rather than the permission.
					Expect(os.Chmod(dir, 0o700)).To(Succeed())
				})

				It("leaves the file the application owns where it is", func() {
					c.Complete()
					Expect(read(wire)).To(Equal("application\n"))
				})
			})
		})
	})

	Describe("the output-file prefix", func() {
		It("starts both names that a Custodian settles", func() {
			prefixed := custody.NewCustodian(dir, "foo_")
			write("foo_"+wire, "application\n")

			Expect(prefixed.SetAside()).To(Succeed())

			Expect(read(backup("foo_" + wire))).To(Equal("application\n"))
			Expect(exists("foo_" + wire)).To(BeFalse())
		})
	})
})

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

package yama_test

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInternalBridgeNotImportableFromExternalModule proves — by construction, not
// by inspection — that Go's internal/ import-path rule forbids code outside the
// l7e.io/yama/v2 module from importing l7e.io/yama/v2/internal/bridge. The fixture
// under testdata/externalmod is its own module and imports internal/bridge; this
// test builds it and asserts the build fails with Go's own visibility error.
//
// This is the module boundary generated application code relies on: generated
// code reaches Lifecycle construction only through the runtime-support package's
// exported API, never through internal/bridge. The external example app later
// exercises the positive side of the same boundary.
func TestInternalBridgeNotImportableFromExternalModule(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skipf("go toolchain not found on PATH: %v", err)
	}

	fixtureDir := filepath.Join("testdata", "externalmod")

	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = fixtureDir
	out, err := cmd.CombinedOutput()
	require.Error(t, err, "expected the external-module build to FAIL on the internal/ import, but it succeeded:\n%s", out)

	got := string(out)
	assert.Contains(t, got, "internal package", "build failed, but not with Go's internal/ visibility error:\n%s", got)
	assert.Contains(t, got, "not allowed", "build failed, but not with Go's internal/ visibility error:\n%s", got)

	t.Logf("external module correctly rejected by Go's internal/ rule:\n%s", strings.TrimSpace(got))
}

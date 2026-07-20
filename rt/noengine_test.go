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

package rt_test

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNoFrameworkDeadline proves the runtime-support package introduces no
// timeout or deadline of its own by scanning its production source: the only
// deadline a lifecycle observes is the one carried by the caller's context.
func TestNoFrameworkDeadline(t *testing.T) {
	entries, err := os.ReadDir(".")
	require.NoError(t, err)

	scanned := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}

		src, err := os.ReadFile(name)
		require.NoError(t, err)

		text := string(src)
		assert.NotContainsf(t, text, "context.WithTimeout", "%s must introduce no framework timeout", name)
		assert.NotContainsf(t, text, "context.WithDeadline", "%s must introduce no framework deadline", name)
		scanned++
	}

	require.Positive(t, scanned, "expected to scan the runtime-support source files")
}

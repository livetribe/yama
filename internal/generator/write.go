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

package generator

import (
	"fmt"
	"os"
	"path/filepath"
)

// lifecycleFileMode is the permission a write requests for the committed
// lifecycle file. The umask applies to it, and a file already in the package
// keeps the permission it carries.
const lifecycleFileMode = 0o666

// Write writes f into its package directory and returns the path it wrote.
//
// A write that fails part way can leave the committed file truncated. Another
// run restores it: generation reads only files that are still on disk, and it
// produces the same content every time.
func (f *LifecycleFile) Write() (string, error) {
	target := filepath.Join(f.Dir, f.Name)

	if err := os.WriteFile(target, f.Content, lifecycleFileMode); err != nil {
		return "", fmt.Errorf("yama: writing %s in %s: %w", f.Name, f.Dir, err)
	}

	return target, nil
}

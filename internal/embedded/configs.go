/*
Copyright © 2022 david amick git@davidamick.com

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package embedded

import (
	"embed"
	"io/fs"
)

//go:generate bash -c "cd ../.. && hack/fetch-default-configs.sh"

//go:embed configs/*
var configFS embed.FS

// GetConfigFiles returns all embedded config files as a map of filename to contents.
func GetConfigFiles() (map[string][]byte, error) {
	files := map[string][]byte{}
	entries, err := fs.ReadDir(configFS, "configs")
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := configFS.ReadFile("configs/" + entry.Name())
		if err != nil {
			return nil, err
		}
		files[entry.Name()] = data
	}

	return files, nil
}

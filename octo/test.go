/*
 * Copyright 2018-2020 the original author or authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package octo

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/paketo-buildpacks/pipeline-builder/octo/actions"
	"github.com/paketo-buildpacks/pipeline-builder/octo/actions/event"
)

func ContributeTest(descriptor Descriptor) (*Contribution, error) {
	if descriptor.OfflinePackages != nil {
		return nil, nil
	}

	w := actions.Workflow{
		Name: "Tests",
		On: map[event.Type]event.Event{
			event.PullRequestType: event.PullRequest{},
			event.PushType: event.Push{
				Branches: []string{"main"},
			},
		},
		Jobs: map[string]actions.Job{},
	}

	if f, err := Find(descriptor.Path, regexp.MustCompile(`.+\.go`).MatchString); err != nil {
		return nil, fmt.Errorf("unable to Find .go files in %s\n%w", descriptor.Path, err)
	} else if len(f) > 0 {
		j := actions.Job{
			Name:   "Unit Test",
			RunsOn: []actions.VirtualEnvironment{actions.UbuntuLatest},
			Steps: []actions.Step{
				{
					Uses: "actions/checkout@v2",
				},
				{
					Uses: "actions/cache@v2",
					With: map[string]interface{}{
						"path":         "${{ env.HOME }}/go/pkg/mod",
						"key":          "${{ runner.os }}-go-${{ hashFiles('**/go.sum') }}",
						"restore-keys": "${{ runner.os }}-go-",
					},
				},
				{
					Uses: "actions/setup-go@v2",
					With: map[string]interface{}{"go-version": GoVersion},
				},
			},
		}

		j.Steps = append(j.Steps, descriptor.Test.Steps...)

		w.Jobs["unit"] = j
	}

	if descriptor.Builder != nil {
		j := actions.Job{
			Name:   "Create Builder Test",
			RunsOn: []actions.VirtualEnvironment{actions.UbuntuLatest},
			Steps: []actions.Step{
				{
					Name: "Install pack",
					Run:  StatikString("/install-pack.sh"),
					Env:  map[string]string{"PACK_VERSION": PackVersion},
				},
				{
					Uses: "actions/checkout@v2",
				},
				{
					Id:   "version",
					Name: "Compute Version",
					Run:  StatikString("/compute-version.sh"),
				},
				{
					Name: "Create Builder",
					Run:  StatikString("/create-builder.sh"),
					Env: map[string]string{
						"BUILDER": "test",
						"VERSION": "${{ steps.version.outputs.version }}",
					},
				},
			},
		}

		j.Steps = append(NewDockerCredentialActions(descriptor.DockerCredentials), j.Steps...)

		w.Jobs["create-builder"] = j
	}

	if descriptor.Package != nil {
		testImageFormat := "image"
		if descriptor.Package.Platform.OS == PlatformWindows {
			testImageFormat = "file"
		}

		j := actions.Job{
			Name:   "Create Package Test",
			RunsOn: []actions.VirtualEnvironment{actions.UbuntuLatest},
			Steps: []actions.Step{
				{
					Name: "Install create-package",
					Run:  StatikString("/install-create-package.sh"),
				},
				{
					Name: "Install pack",
					Run:  StatikString("/install-pack.sh"),
					Env:  map[string]string{"PACK_VERSION": PackVersion},
				},
				{
					Name: "Enable pack Experimental",
					If:   fmt.Sprintf("${{ %t }}", descriptor.Package.Platform.OS == PlatformWindows),
					Run:  StatikString("/enable-pack-experimental.sh"),
				},
				{
					Uses: "actions/setup-go@v2",
					With: map[string]interface{}{"go-version": GoVersion},
				},
				{
					Uses: "actions/checkout@v2",
				},
				{
					Uses: "actions/cache@v2",
					With: map[string]interface{}{
						"path": strings.Join([]string{
							"${{ env.HOME }}/.pack",
							"${{ env.HOME }}/carton-cache",
						}, "\n"),
						"key":          "${{ runner.os }}-go-${{ hashFiles('**/buildpack.toml', '**/package.toml') }}",
						"restore-keys": "${{ runner.os }}-go-",
					},
				},
				{
					Id:   "version",
					Name: "Compute Version",
					Run:  StatikString("/compute-version.sh"),
				},
				{
					Name: "Create Package",
					Run:  StatikString("/create-package.sh"),
					Env: map[string]string{
						"INCLUDE_DEPENDENCIES": "true",
						"OS":                   descriptor.Package.Platform.OS,
						"VERSION":              "${{ steps.version.outputs.version }}",
					},
				},
				{
					Name: "Package Buildpack",
					Run:  StatikString("/package-buildpack.sh"),
					Env: map[string]string{
						"PACKAGE": "test",
						"FORMAT": testImageFormat,
						"VERSION": "${{ steps.version.outputs.version }}",
					},
				},
			},
		}

		j.Steps = append(NewDockerCredentialActions(descriptor.DockerCredentials), j.Steps...)
		j.Steps = append(NewHttpCredentialActions(descriptor.HttpCredentials), j.Steps...)

		w.Jobs["create-package"] = j
	}

	c, err := NewActionContribution(w)
	if err != nil {
		return nil, err
	}

	return &c, nil
}

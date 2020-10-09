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
	"github.com/paketo-buildpacks/pipeline-builder/octo/actions"
	"github.com/paketo-buildpacks/pipeline-builder/octo/actions/event"
	"github.com/paketo-buildpacks/pipeline-builder/octo/internal"
)

func ContributeCreateBuilder(descriptor Descriptor) (*Contribution, error) {
	if descriptor.Builder == nil {
		return nil, nil
	}

	w := actions.Workflow{
		Name: "Create Builder",
		On: map[event.Type]event.Event{
			event.ReleaseType: event.Release{
				Types: []event.ReleaseActivityType{
					event.ReleasePublished,
				},
			},
		},
		Jobs: map[string]actions.Job{
			"create-builder": {
				Name:   "Create Builder",
				RunsOn: []actions.VirtualEnvironment{actions.UbuntuLatest},
				Steps: []actions.Step{
					{
						Uses: "actions/checkout@v2",
					},
					{
						Name: "Install pack",
						Run:  internal.StatikString("/install-pack.sh"),
						Env:  map[string]string{"PACK_VERSION": PackVersion},
					},
					{
						Id:   "version",
						Name: "Compute Version",
						Run:  internal.StatikString("/compute-version.sh"),
					},
					{
						Name: "Create Builder",
						Run:  internal.StatikString("/create-builder.sh"),
						Env: map[string]string{
							"BUILDER": descriptor.Builder.Repository,
							"PUBLISH": "true",
							"VERSION": "${{ steps.version.outputs.version }}",
						},
					},
				},
			},
		},
	}

	j := w.Jobs["create-builder"]
	j.Steps = append(NewDockerLoginActions(descriptor.Credentials), j.Steps...)
	w.Jobs["create-builder"] = j

	c, err := NewActionContribution(w)
	if err != nil {
		return nil, err
	}

	return &c, nil
}
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

package tube

import (
	"context"
	"fmt"
	"regexp"

	"github.com/BurntSushi/toml"
	"github.com/google/go-github/v30/github"
)

type UpdateBuilderDependencyContributor struct {
	Descriptor Descriptor
	Package    string
	Salt       string
}

func NewUpdateBuilderDependencyContributors(descriptor Descriptor, salt string, gh *github.Client) ([]UpdateBuilderDependencyContributor, error) {
	in, err := gh.Repositories.DownloadContents(context.Background(), descriptor.Owner(), descriptor.Repository(), "builder.toml", nil)
	if err != nil {
		return nil, fmt.Errorf("unable to get %s/go.mod\n%w", descriptor.Name, err)
	}
	defer in.Close()

	d := struct {
		Buildpacks []struct {
			Image string `toml:"image"`
		} `toml:"buildpacks"`
	}{}

	if _, err := toml.DecodeReader(in, &d); err != nil {
		return nil, fmt.Errorf("unable to decode\n%w", err)
	}

	re := regexp.MustCompile(`(?m)^(.+):[^:]+$`)
	var b []UpdateBuilderDependencyContributor
	for _, bp := range d.Buildpacks {
		if s := re.FindStringSubmatch(bp.Image); s != nil {
			b = append(b, UpdateBuilderDependencyContributor{
				Descriptor: descriptor,
				Package:    s[1],
				Salt:       salt,
			})
		}
	}

	return b, nil
}

func (UpdateBuilderDependencyContributor) Group() string {
	return "dependency"
}

func (u UpdateBuilderDependencyContributor) Job() Job {
	b := NewBuildCommonResource()
	d := NewBuilderDependencyResource(u.Package)
	s := NewSourceResource(u.Descriptor, u.Salt)

	return Job{
		Name:   d.Name,
		Public: true,
		Plan: []map[string]interface{}{
			{
				"in_parallel": []map[string]interface{}{
					{
						"get":      "build-common",
						"resource": b.Name,
					},
					{
						"get":      "dependency",
						"resource": d.Name,
						"trigger":  true,
					},
					{
						"get":      "source",
						"resource": s.Name,
					},
				},
			},
			{
				"task": "update-builder-dependency",
				"file": "build-common/update-builder-dependency.yml",
				"params": map[string]interface{}{
					"DEPENDENCY": u.Package,
				},
			},
			{
				"put": s.Name,
				"params": map[string]interface{}{
					"repository": "source",
					"rebase":     true,
				},
			},
		},
	}

}

func (u UpdateBuilderDependencyContributor) Resources() []Resource {
	return []Resource{
		NewBuildCommonResource(),
		NewBuilderDependencyResource(u.Package),
		NewSourceResource(u.Descriptor, u.Salt),
	}
}
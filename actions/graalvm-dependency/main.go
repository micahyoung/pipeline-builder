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

package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"sort"

	"github.com/Masterminds/semver/v3"
	"github.com/google/go-github/v32/github"
	"golang.org/x/oauth2"

	"github.com/paketo-buildpacks/pipeline-builder/actions"
)

func main() {
	inputs := actions.NewInputs()

	var (
		err error
		g   *regexp.Regexp
	)
	if s, ok := inputs["glob"]; ok {
		g, err = regexp.Compile(s)
		if err != nil {
			panic(fmt.Errorf("unable to compile %s as a regexp\n%w", s, err))
		}
	} else {
		g = regexp.MustCompile(".+")
	}

	v, ok := inputs["version"]
	if !ok {
		panic(fmt.Errorf("repository must be specified"))
	}

	var c *http.Client
	if s, ok := inputs["token"]; ok {
		c = oauth2.NewClient(context.Background(), oauth2.StaticTokenSource(&oauth2.Token{AccessToken: s}))
	}
	gh := github.NewClient(c)

	candidates := make(map[*semver.Version]Holder)
	re := regexp.MustCompile(`vm-(.+)`)
	opt := &github.ListOptions{PerPage: 100}
	for {
		rel, rsp, err := gh.Repositories.ListReleases(context.Background(), "graalvm", "graalvm-ce-builds", opt)
		if err != nil {
			panic(fmt.Errorf("unable to list existing releases for graalvm/graalvm-ce-builds\n%w", err))
		}

		for _, r := range rel {
			for _, a := range r.Assets {
				if g.MatchString(*a.Name) {
					if g := re.FindStringSubmatch(*r.TagName); g != nil {
						v, err := actions.NormalizeVersion(g[1])
						if err != nil {
							panic(err)
						}

						sv, err := semver.NewVersion(v)
						if err != nil {
							panic(fmt.Errorf("unable to parse %s as semver", g[1]))
						}

						candidates[sv] = Holder{Assets: r.Assets, URI: *a.BrowserDownloadURL}
						break
					}
				}
			}
		}

		if rsp.NextPage == 0 {
			break
		}
		opt.Page = rsp.NextPage
	}

	var sv []*semver.Version
	for k := range candidates {
		sv = append(sv, k)
	}

	sort.Slice(sv, func(i, j int) bool {
		return sv[i].LessThan(sv[j])
	})

	h := candidates[sv[len(sv)-1]]
	versions := actions.Versions{GetVersion(h.Assets, v): h.URI}

	if o, err := versions.GetLatest(inputs); err != nil {
		panic(err)
	} else {
		o.Write(os.Stdout)
	}
}

func GetVersion(assets []*github.ReleaseAsset, version string) string {
	re := regexp.MustCompile(fmt.Sprintf(`graalvm-ce-java%s-linux-amd64-.+.tar.gz`, version))

	var uri *string

	for _, a := range assets {
		if re.MatchString(*a.Name) {
			uri = a.BrowserDownloadURL
			break
		}
	}

	if uri == nil {
		panic(fmt.Errorf("unable to find asset that matches %s", re.String()))
	}

	resp, err := http.Get(*uri)
	if err != nil {
		panic(fmt.Errorf("unable to get %s\n%w", *uri, err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		panic(fmt.Errorf("unable to download %s: %d", *uri, resp.StatusCode))
	}

	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		panic(fmt.Errorf("unable to create GZIP reader\n%w", err))
	}
	defer gz.Close()

	t := tar.NewReader(gz)

	re = regexp.MustCompile(fmt.Sprintf(`graalvm-ce-java%s-[^/]+/release`, version))
	for {
		f, err := t.Next()
		if err != nil && err == io.EOF {
			break
		} else if err != nil {
			panic(fmt.Errorf("unable to read TAR file\n%w", err))
		}

		if !re.MatchString(f.Name) {
			continue
		}

		b, err := ioutil.ReadAll(t)
		if err != nil {
			panic(fmt.Errorf("unable to read %s\n%w", f.Name, err))
		}

		var v string

		re = regexp.MustCompile(`JAVA_VERSION="([\d]+)\.([\d]+)\.([\d]+)[_]?([\d]+)?"`)
		if g := re.FindStringSubmatch(string(b)); g != nil {
			if g[2] == "8" {
				v = fmt.Sprintf("8.0.%s", g[4])
			} else {
				v = fmt.Sprintf("%s.%s.%s", g[1], g[2], g[3])
			}
		}

		return v
	}

	panic(fmt.Errorf("unable to find file that matches %s", re.String()))
}

type Holder struct {
	Assets []*github.ReleaseAsset
	URI    string
}

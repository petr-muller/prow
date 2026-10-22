/*
Copyright 2020 The Kubernetes Authors.

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

// Package junit provides a junit viewer for Spyglass
package junit

import (
	"bytes"
	"context"
	"html/template"
	"io"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/testgrid/metadata/junit"
	"github.com/google/go-cmp/cmp"

	"sigs.k8s.io/prow/pkg/spyglass/api"
	"sigs.k8s.io/prow/pkg/spyglass/lenses"
)

const (
	fakeCanonicalLink = "linknotfound.io/404"
)

// FakeArtifact implements lenses.Artifact.
// This is pretty much copy/pasted from prow/spyglass/lenses/lenses_test.go, if
// another package needs to reuse, should think about refactor this into it's
// own package
type FakeArtifact struct {
	path      string
	content   []byte
	sizeLimit int64
}

func (fa *FakeArtifact) JobPath() string {
	return fa.path
}

func (fa *FakeArtifact) Size() (int64, error) {
	return int64(len(fa.content)), nil
}

func (fa *FakeArtifact) CanonicalLink() string {
	return fakeCanonicalLink
}

func (fa *FakeArtifact) Metadata() (map[string]string, error) {
	return nil, nil
}

func (fa *FakeArtifact) UpdateMetadata(map[string]string) error {
	return nil
}

func (fa *FakeArtifact) ReadAt(b []byte, off int64) (int, error) {
	r := bytes.NewReader(fa.content)
	return r.ReadAt(b, off)
}

func (fa *FakeArtifact) ReadAll() ([]byte, error) {
	size, err := fa.Size()
	if err != nil {
		return nil, err
	}
	if size > fa.sizeLimit {
		return nil, lenses.ErrFileTooLarge
	}
	r := bytes.NewReader(fa.content)
	return io.ReadAll(r)
}

func (fa *FakeArtifact) ReadTail(n int64) ([]byte, error) {
	return nil, nil
}

func (fa *FakeArtifact) UseContext(ctx context.Context) error {
	return nil
}

func (fa *FakeArtifact) ReadAtMost(n int64) ([]byte, error) {
	return nil, nil
}

func TestJunitResultStatusAttribute(t *testing.T) {
	for _, tc := range []struct {
		name, attributes, outcome string
		want                      testStatus
	}{
		{name: "skipped attribute", attributes: `status="skipped"`, want: skippedStatus},
		{name: "missing attribute", want: passedStatus},
		{name: "unknown attribute", attributes: `status="unknown"`, want: passedStatus},
		{name: "case sensitive", attributes: `status="Skipped"`, want: passedStatus},
		{name: "whitespace significant", attributes: `status=" skipped "`, want: passedStatus},
		{name: "failure overrides attribute", attributes: `status="skipped"`, outcome: `<failure/>`, want: failedStatus},
		{name: "error overrides attribute", attributes: `status="skipped"`, outcome: `<error/>`, want: failedStatus},
		{name: "explicit skipped overrides unknown", attributes: `status="unknown"`, outcome: `<skipped/>`, want: skippedStatus},
		{name: "explicit skipped overrides failure and error", attributes: `status="skipped"`, outcome: `<skipped/><failure/><error/>`, want: skippedStatus},
	} {
		t.Run(tc.name, func(t *testing.T) {
			suites, err := junit.Parse([]byte(`<testsuites><testsuite><testcase name="test" ` + tc.attributes + `>` + tc.outcome + `</testcase></testsuite></testsuites>`))
			if err != nil {
				t.Fatal(err)
			}
			if len(suites.Suites) != 1 || len(suites.Suites[0].Results) != 1 {
				t.Fatalf("expected one parsed test result, got %+v", suites)
			}
			if got := (JunitResult{Result: suites.Suites[0].Results[0]}).Status(); got != tc.want {
				t.Errorf("Status() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestGetJvdStatusAttribute(t *testing.T) {
	type counts struct {
		total, passed, failed, skipped, flaky int
	}
	for _, tc := range []struct {
		name string
		runs []string
		want counts
	}{
		{name: "skipped", runs: []string{`status="skipped">`}, want: counts{total: 1, skipped: 1}},
		{name: "repeated skipped", runs: []string{`status="skipped">`, `status="skipped">`}, want: counts{total: 1, skipped: 1}},
		{name: "skip then pass", runs: []string{`status="skipped">`, `>`}, want: counts{total: 1, skipped: 1}},
		{name: "pass then skip", runs: []string{`>`, `status="skipped">`}, want: counts{total: 1, skipped: 1}},
		{name: "skip then fail", runs: []string{`status="skipped">`, `><failure/>`}, want: counts{total: 1, failed: 1, skipped: 1}},
		{name: "fail then skip", runs: []string{`><failure/>`, `status="skipped">`}, want: counts{total: 1, failed: 1, skipped: 1}},
		{name: "skip with pass and fail", runs: []string{`status="skipped">`, `>`, `><failure/>`}, want: counts{total: 1, skipped: 1}},
		{name: "pass and fail remain flaky", runs: []string{`>`, `><failure/>`}, want: counts{total: 1, flaky: 1}},
		{name: "failure overrides attribute", runs: []string{`status="skipped"><failure/>`}, want: counts{total: 1, failed: 1}},
		{name: "missing attribute passes", runs: []string{`>`}, want: counts{total: 1, passed: 1}},
		{name: "unknown attribute passes", runs: []string{`status="other">`}, want: counts{total: 1, passed: 1}},
	} {
		for _, grouped := range []bool{false, true} {
			name := tc.name + "/default"
			var groups []GroupConfig
			if grouped {
				name = tc.name + "/configured group"
				groups = []GroupConfig{{Name: "Informing", Selector: "properties/property[@name='lifecycle' and @value='informing']"}}
			}
			t.Run(name, func(t *testing.T) {
				var xml strings.Builder
				xml.WriteString(`<testsuites><testsuite>`)
				for _, run := range tc.runs {
					xml.WriteString(`<testcase name="test" ` + run + `<properties><property name="lifecycle" value="informing"/></properties></testcase>`)
				}
				xml.WriteString(`</testsuite></testsuites>`)
				got := (Lens{}).getJvd([]api.Artifact{&FakeArtifact{path: "junit.xml", content: []byte(xml.String()), sizeLimit: 500e6}}, groups)
				actual := counts{got.NumTests, len(got.Passed), len(got.Failed), len(got.Skipped), len(got.Flaky)}
				if grouped {
					if actual != (counts{}) {
						t.Errorf("default counts = %+v, want empty", actual)
					}
					if len(got.Groups) != 1 {
						t.Fatalf("got %d groups, want 1", len(got.Groups))
					}
					g := got.Groups[0]
					actual = counts{g.NumTests, len(g.Passed), len(g.Failed), len(g.Skipped), len(g.Flaky)}
				}
				if actual != tc.want {
					t.Errorf("counts = %+v, want %+v", actual, tc.want)
				}
			})
		}
	}
}

func TestGetJvd(t *testing.T) {
	emptySkipped := junit.Skipped{}
	failures := []junit.Failure{
		{
			Type:    "failure",
			Message: "failure message 0",
			Value:   " failure value 0 ",
		},
		{
			Type:    "failure",
			Message: "failure message 1",
			Value:   " failure value 1 ",
		},
	}
	errors := []junit.Errored{
		{
			Type:    "error",
			Message: "error message 0",
			Value:   " error value 0 ",
		},
		{
			Type:    "error",
			Message: "error message 1",
			Value:   " error value 1 ",
		},
	}

	tests := []struct {
		name       string
		rawResults [][]byte
		exp        JVD
	}{
		{
			"Failed",
			[][]byte{
				[]byte(`
				<testsuites>
					<testsuite>
						<testcase classname="fake_class_0" name="fake_test_0">
							<failure message="failure message 0" type="failure"> failure value 0 </failure>
						</testcase>
					</testsuite>
				</testsuites>
				`),
			},
			JVD{
				NumTests: 1,
				Passed:   nil,
				Failed: []TestResult{
					{
						Junit: []JunitResult{
							{
								junit.Result{
									Name:      "fake_test_0",
									ClassName: "fake_class_0",
									Failure:   &failures[0],
								},
							},
						},
						Link: "linknotfound.io/404",
					},
				},
				Skipped: nil,
				Flaky:   nil,
			},
		}, {
			"Errored",
			[][]byte{
				[]byte(`
				<testsuites>
					<testsuite>
						<testcase classname="fake_class_0" name="fake_test_0">
							<error message="error message 0" type="error"> error value 0 </error>
						</testcase>
					</testsuite>
				</testsuites>
				`),
			},
			JVD{
				NumTests: 1,
				Passed:   nil,
				Failed: []TestResult{
					{
						Junit: []JunitResult{
							{
								junit.Result{
									Name:      "fake_test_0",
									ClassName: "fake_class_0",
									Errored:   &errors[0],
								},
							},
						},
						Link: "linknotfound.io/404",
					},
				},
				Skipped: nil,
				Flaky:   nil,
			},
		}, {
			"Passed",
			[][]byte{
				[]byte(`
				<testsuites>
					<testsuite>
						<testcase classname="fake_class_0" name="fake_test_0"></testcase>
					</testsuite>
				</testsuites>
				`),
			},
			JVD{
				NumTests: 1,
				Passed: []TestResult{
					{
						Junit: []JunitResult{
							{
								junit.Result{
									Name:      "fake_test_0",
									ClassName: "fake_class_0",
									Failure:   nil,
								},
							},
						},
						Link: "linknotfound.io/404",
					},
				},
				Failed:  nil,
				Skipped: nil,
				Flaky:   nil,
			},
		}, {
			"Skipped",
			[][]byte{
				[]byte(`
				<testsuites>
					<testsuite>
						<testcase classname="fake_class_0" name="fake_test_0">
							<skipped/>
						</testcase>
					</testsuite>
				</testsuites>
				`),
			},
			JVD{
				NumTests: 1,
				Passed:   nil,
				Failed:   nil,
				Skipped: []TestResult{
					{
						Junit: []JunitResult{
							{
								junit.Result{
									Name:      "fake_test_0",
									ClassName: "fake_class_0",
									Failure:   nil,
									Skipped:   &emptySkipped,
								},
							},
						},
						Link: "linknotfound.io/404",
					},
				},
				Flaky: nil,
			},
		}, {
			"Failed and Skipped",
			[][]byte{
				[]byte(`
				<testsuites>
					<testsuite>
						<testcase classname="fake_class_0" name="fake_test_0">
							<failure message="failure message 0" type="failure"> failure value 0 </failure>
						</testcase>
					<testcase classname="fake_class_0" name="fake_test_0">
							<skipped/>
						</testcase>
					</testsuite>
				</testsuites>
				`),
			},
			JVD{
				NumTests: 1,
				Passed:   nil,
				Failed: []TestResult{
					{
						Junit: []JunitResult{
							{
								junit.Result{
									Name:      "fake_test_0",
									ClassName: "fake_class_0",
									Failure:   &failures[0],
								},
							},
							{
								junit.Result{
									Name:      "fake_test_0",
									ClassName: "fake_class_0",
									Failure:   nil,
									Skipped:   &emptySkipped,
								},
							},
						},
						Link: "linknotfound.io/404",
					},
				},
				Skipped: []TestResult{
					{
						Junit: []JunitResult{
							{
								junit.Result{
									Name:      "fake_test_0",
									ClassName: "fake_class_0",
									Failure:   &failures[0],
								},
							},
							{
								junit.Result{
									Name:      "fake_test_0",
									ClassName: "fake_class_0",
									Failure:   nil,
									Skipped:   &emptySkipped,
								},
							},
						},
						Link: "linknotfound.io/404",
					},
				},
				Flaky: nil,
			},
		}, {
			"Multiple tests in same file",
			[][]byte{
				[]byte(`
				<testsuites>
					<testsuite>
						<testcase classname="fake_class_0" name="fake_test_0">
							<failure message="failure message 0" type="failure"> failure value 0 </failure>
						</testcase>
						<testcase classname="fake_class_1" name="fake_test_0"></testcase>
					</testsuite>
				</testsuites>
				`),
			},
			JVD{
				NumTests: 2,
				Passed: []TestResult{
					{
						Junit: []JunitResult{
							{
								junit.Result{
									Name:      "fake_test_0",
									ClassName: "fake_class_1",
									Failure:   nil,
								},
							},
						},
						Link: "linknotfound.io/404",
					},
				},
				Failed: []TestResult{
					{
						Junit: []JunitResult{
							{
								junit.Result{
									Name:      "fake_test_0",
									ClassName: "fake_class_0",
									Failure:   &failures[0],
								},
							},
						},
						Link: "linknotfound.io/404",
					},
				},
				Skipped: nil,
				Flaky:   nil,
			},
		}, {
			"Multiple tests in different files",
			[][]byte{
				[]byte(`
				<testsuites>
					<testsuite>
						<testcase classname="fake_class_0" name="fake_test_0">
							<failure message="failure message 0" type="failure"> failure value 0 </failure>
						</testcase>
					</testsuite>
				</testsuites>
				`),
				[]byte(`
				<testsuites>
					<testsuite>
						<testcase classname="fake_class_1" name="fake_test_0"></testcase>
					</testsuite>
				</testsuites>
				`),
			},
			JVD{
				NumTests: 2,
				Passed: []TestResult{
					{
						Junit: []JunitResult{
							{
								junit.Result{
									Name:      "fake_test_0",
									ClassName: "fake_class_1",
									Failure:   nil,
								},
							},
						},
						Link: "linknotfound.io/404",
					},
				},
				Failed: []TestResult{
					{
						Junit: []JunitResult{
							{
								junit.Result{
									Name:      "fake_test_0",
									ClassName: "fake_class_0",
									Failure:   &failures[0],
								},
							},
						},
						Link: "linknotfound.io/404",
					},
				},
				Skipped: nil,
				Flaky:   nil,
			},
		}, {
			"Fail multiple times in same file",
			[][]byte{
				[]byte(`
				<testsuites>
					<testsuite>
						<testcase classname="fake_class_0" name="fake_test_0">
							<failure message="failure message 0" type="failure"> failure value 0 </failure>
						</testcase>
					</testsuite>
					<testsuite>
						<testcase classname="fake_class_0" name="fake_test_0">
							<failure message="failure message 1" type="failure"> failure value 1 </failure>
						</testcase>
					</testsuite>
				</testsuites>
				`),
			},
			JVD{
				NumTests: 1,
				Passed:   nil,
				Failed: []TestResult{
					{
						Junit: []JunitResult{
							{
								junit.Result{
									Name:      "fake_test_0",
									ClassName: "fake_class_0",
									Failure:   &failures[0],
								},
							},
							{
								junit.Result{
									Name:      "fake_test_0",
									ClassName: "fake_class_0",
									Failure:   &failures[1],
								},
							},
						},
						Link: "linknotfound.io/404",
					},
				},
				Skipped: nil,
				Flaky:   nil,
			},
		}, {
			"Passed multiple times in same file",
			[][]byte{
				[]byte(`
				<testsuites>
					<testsuite>
						<testcase classname="fake_class_0" name="fake_test_0"></testcase>
					</testsuite>
					<testsuite>
						<testcase classname="fake_class_0" name="fake_test_0"></testcase>
					</testsuite>
				</testsuites>
				`),
			},
			JVD{
				NumTests: 1,
				Passed: []TestResult{
					{
						Junit: []JunitResult{
							{
								junit.Result{
									Name:      "fake_test_0",
									ClassName: "fake_class_0",
									Failure:   nil,
								},
							},
							{
								junit.Result{
									Name:      "fake_test_0",
									ClassName: "fake_class_0",
									Failure:   nil,
								},
							},
						},
						Link: "linknotfound.io/404",
					},
				},
				Failed:  nil,
				Skipped: nil,
				Flaky:   nil,
			},
		}, {
			// This is the case where `go test --count=N`, where N>1
			"Passed multiple times in same suite",
			[][]byte{
				[]byte(`
				<testsuites>
					<testsuite>
						<testcase classname="fake_class_0" name="fake_test_0"></testcase>
						<testcase classname="fake_class_0" name="fake_test_0"></testcase>
					</testsuite>
				</testsuites>
				`),
			},
			JVD{
				NumTests: 1,
				Passed: []TestResult{
					{
						Junit: []JunitResult{
							{
								junit.Result{
									Name:      "fake_test_0",
									ClassName: "fake_class_0",
									Failure:   nil,
								},
							},
							{
								junit.Result{
									Name:      "fake_test_0",
									ClassName: "fake_class_0",
									Failure:   nil,
								},
							},
						},
						Link: "linknotfound.io/404",
					},
				},
				Failed:  nil,
				Skipped: nil,
				Flaky:   nil,
			},
		}, {
			"Failed then pass in same file (flaky)",
			[][]byte{
				[]byte(`
				<testsuites>
					<testsuite>
						<testcase classname="fake_class_0" name="fake_test_0">
							<failure message="failure message 0" type="failure"> failure value 0 </failure>
						</testcase>
					</testsuite>
					<testsuite>
						<testcase classname="fake_class_0" name="fake_test_0"></testcase>
					</testsuite>
				</testsuites>
				`),
			},
			JVD{
				NumTests: 1,
				Passed:   nil,
				Failed:   nil,
				Skipped:  nil,
				Flaky: []TestResult{
					{
						Junit: []JunitResult{
							{
								junit.Result{
									Name:      "fake_test_0",
									ClassName: "fake_class_0",
									Failure:   &failures[0],
								},
							},
							{
								junit.Result{
									Name:      "fake_test_0",
									ClassName: "fake_class_0",
									Failure:   nil,
								},
							},
						},
						Link: "linknotfound.io/404",
					},
				},
			},
		}, {
			"Pass then fail in same file (flaky)",
			[][]byte{
				[]byte(`
				<testsuites>
					<testsuite>
						<testcase classname="fake_class_0" name="fake_test_0"></testcase>
					</testsuite>
					<testsuite>
						<testcase classname="fake_class_0" name="fake_test_0">
							<failure message="failure message 0" type="failure"> failure value 0 </failure>
						</testcase>
					</testsuite>
				</testsuites>
				`),
			},
			JVD{
				NumTests: 1,
				Passed:   nil,
				Failed:   nil,
				Skipped:  nil,
				Flaky: []TestResult{
					{
						Junit: []JunitResult{
							{
								junit.Result{
									Name:      "fake_test_0",
									ClassName: "fake_class_0",
									Failure:   nil,
								},
							},
							{
								junit.Result{
									Name:      "fake_test_0",
									ClassName: "fake_class_0",
									Failure:   &failures[0],
								},
							},
						},
						Link: "linknotfound.io/404",
					},
				},
			},
		}, {
			"Fail multiple times then pass in same file (flaky)",
			[][]byte{
				[]byte(`
				<testsuites>
					<testsuite>
						<testcase classname="fake_class_0" name="fake_test_0">
							<failure message="failure message 0" type="failure"> failure value 0 </failure>
						</testcase>
					</testsuite>
					<testsuite>
						<testcase classname="fake_class_0" name="fake_test_0">
							<failure message="failure message 1" type="failure"> failure value 1 </failure>
						</testcase>
					</testsuite>
					<testsuite>
						<testcase classname="fake_class_0" name="fake_test_0"></testcase>
					</testsuite>
				</testsuites>
				`),
			},
			JVD{
				NumTests: 1,
				Passed:   nil,
				Failed:   nil,
				Skipped:  nil,
				Flaky: []TestResult{
					{
						Junit: []JunitResult{
							{
								junit.Result{
									Name:      "fake_test_0",
									ClassName: "fake_class_0",
									Failure:   &failures[0],
								},
							},
							{
								junit.Result{
									Name:      "fake_test_0",
									ClassName: "fake_class_0",
									Failure:   &failures[1],
								},
							},
							{
								junit.Result{
									Name:      "fake_test_0",
									ClassName: "fake_class_0",
									Failure:   nil,
								},
							},
						},
						Link: "linknotfound.io/404",
					},
				},
			},
		}, {
			"Same test in different file",
			[][]byte{
				[]byte(`
				<testsuites>
					<testsuite>
						<testcase classname="fake_class_0" name="fake_test_0">
							<failure message="failure message 0" type="failure"> failure value 0 </failure>
						</testcase>
					</testsuite>
				</testsuites>
				`),
				[]byte(`
				<testsuites>
					<testsuite>
						<testcase classname="fake_class_0" name="fake_test_0"></testcase>
					</testsuite>
				</testsuites>
				`),
			},
			JVD{
				NumTests: 2,
				Passed: []TestResult{
					{
						Junit: []JunitResult{
							{
								junit.Result{
									Name:      "fake_test_0",
									ClassName: "fake_class_0",
									Failure:   nil,
								},
							},
						},
						Link: "linknotfound.io/404",
					},
				},
				Failed: []TestResult{
					{
						Junit: []JunitResult{
							{
								junit.Result{
									Name:      "fake_test_0",
									ClassName: "fake_class_0",
									Failure:   &failures[0],
								},
							},
						},
						Link: "linknotfound.io/404",
					},
				},
				Skipped: nil,
				Flaky:   nil,
			},
		}, {
			"Sequence of test cases in the artifact file is reflected in the lens",
			[][]byte{
				[]byte(`
				<testsuites>
					<testsuite>
						<testcase classname="fake_class_0" name="fake_test_0"></testcase>
					</testsuite>
					<testsuite>
						<testcase classname="fake_class_1" name="fake_test_1"></testcase>
					</testsuite>
					<testsuite>
						<testcase classname="fake_class_2" name="fake_test_2"></testcase>
					</testsuite>
					<testsuite>
						<testcase classname="fake_class_3" name="fake_test_3"></testcase>
					</testsuite>
					<testsuite>
						<testcase classname="fake_class_4" name="fake_test_4"></testcase>
					</testsuite>
				</testsuites>
				`),
			},
			JVD{
				NumTests: 5,
				Passed: []TestResult{
					{
						Junit: []JunitResult{
							{
								junit.Result{
									Name:      "fake_test_0",
									ClassName: "fake_class_0",
									Failure:   nil,
								},
							},
						},
						Link: "linknotfound.io/404",
					},
					{
						Junit: []JunitResult{
							{
								junit.Result{
									Name:      "fake_test_1",
									ClassName: "fake_class_1",
									Failure:   nil,
								},
							},
						},
						Link: "linknotfound.io/404",
					},
					{
						Junit: []JunitResult{
							{
								junit.Result{
									Name:      "fake_test_2",
									ClassName: "fake_class_2",
									Failure:   nil,
								},
							},
						},
						Link: "linknotfound.io/404",
					},
					{
						Junit: []JunitResult{
							{
								junit.Result{
									Name:      "fake_test_3",
									ClassName: "fake_class_3",
									Failure:   nil,
								},
							},
						},
						Link: "linknotfound.io/404",
					},
					{
						Junit: []JunitResult{
							{
								junit.Result{
									Name:      "fake_test_4",
									ClassName: "fake_class_4",
									Failure:   nil,
								},
							},
						},
						Link: "linknotfound.io/404",
					},
				},
				Failed:  nil,
				Skipped: nil,
				Flaky:   nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			artifacts := make([]api.Artifact, 0)
			for _, rr := range tt.rawResults {
				artifacts = append(artifacts, &FakeArtifact{
					path:      "log.txt",
					content:   rr,
					sizeLimit: 500e6,
				})
			}
			l := Lens{}
			got := l.getJvd(artifacts, nil)
			if diff := cmp.Diff(tt.exp, got); diff != "" {
				t.Fatalf("JVD mismatch, want(-), got(+): \n%s", diff)
			}
		})
	}
}

func TestParseSelector(t *testing.T) {
	tests := []struct {
		name      string
		selector  string
		wantErr   bool
		wantProps []propertyPredicate
	}{
		{
			name:     "name and value",
			selector: "properties/property[@name='lifecycle' and @value='informing']",
			wantProps: []propertyPredicate{
				{name: "lifecycle", value: "informing"},
			},
		},
		{
			name:     "name only",
			selector: "properties/property[@name='lifecycle']",
			wantProps: []propertyPredicate{
				{name: "lifecycle"},
			},
		},
		{
			name:     "value only",
			selector: "properties/property[@value='informing']",
			wantProps: []propertyPredicate{
				{value: "informing"},
			},
		},
		{
			name:     "unsupported selector",
			selector: "classname='foo'",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pred, err := parseSelector(tt.selector)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if diff := cmp.Diff(tt.wantProps, pred.properties, cmp.AllowUnexported(propertyPredicate{})); diff != "" {
				t.Fatalf("predicate mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSelectorMatches(t *testing.T) {
	tests := []struct {
		name     string
		selector string
		result   junit.Result
		want     bool
	}{
		{
			name:     "matches property name and value",
			selector: "properties/property[@name='lifecycle' and @value='informing']",
			result: junit.Result{
				Properties: &junit.Properties{
					PropertyList: []junit.Property{
						{Name: "lifecycle", Value: "informing"},
					},
				},
			},
			want: true,
		},
		{
			name:     "does not match different value",
			selector: "properties/property[@name='lifecycle' and @value='informing']",
			result: junit.Result{
				Properties: &junit.Properties{
					PropertyList: []junit.Property{
						{Name: "lifecycle", Value: "blocking"},
					},
				},
			},
			want: false,
		},
		{
			name:     "does not match nil properties",
			selector: "properties/property[@name='lifecycle' and @value='informing']",
			result:   junit.Result{},
			want:     false,
		},
		{
			name:     "matches name only selector",
			selector: "properties/property[@name='lifecycle']",
			result: junit.Result{
				Properties: &junit.Properties{
					PropertyList: []junit.Property{
						{Name: "lifecycle", Value: "anything"},
					},
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pred, err := parseSelector(tt.selector)
			if err != nil {
				t.Fatalf("failed to parse selector: %v", err)
			}
			got := pred.matches(tt.result)
			if got != tt.want {
				t.Fatalf("matches() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetJvdWithGroups(t *testing.T) {
	tests := []struct {
		name       string
		rawResults [][]byte
		groups     []GroupConfig
		expFailed  int
		expPassed  int
		expGroups  []struct {
			name    string
			failed  int
			passed  int
			skipped int
		}
	}{
		{
			name: "informing tests go to group with full breakdown",
			rawResults: [][]byte{
				[]byte(`
				<testsuites>
					<testsuite>
						<testcase classname="cls" name="blocking_test">
							<failure message="failure message 0" type="failure"> failure value 0 </failure>
						</testcase>
						<testcase classname="cls" name="blocking_pass"></testcase>
						<testcase classname="cls" name="informing_fail">
							<properties>
								<property name="lifecycle" value="informing"/>
							</properties>
							<failure message="failure message 1" type="failure"> failure value 1 </failure>
						</testcase>
						<testcase classname="cls" name="informing_pass">
							<properties>
								<property name="lifecycle" value="informing"/>
							</properties>
						</testcase>
						<testcase classname="cls" name="informing_skip">
							<properties>
								<property name="lifecycle" value="informing"/>
							</properties>
							<skipped/>
						</testcase>
					</testsuite>
				</testsuites>
				`),
			},
			groups: []GroupConfig{
				{
					Name:      "Informing",
					Selector:  "properties/property[@name='lifecycle' and @value='informing']",
					Collapsed: true,
				},
			},
			expFailed: 1,
			expPassed: 1,
			expGroups: []struct {
				name    string
				failed  int
				passed  int
				skipped int
			}{
				{name: "Informing", failed: 1, passed: 1, skipped: 1},
			},
		},
		{
			name: "no groups configured, all tests stay in default",
			rawResults: [][]byte{
				[]byte(`
				<testsuites>
					<testsuite>
						<testcase classname="cls" name="test1">
							<properties>
								<property name="lifecycle" value="informing"/>
							</properties>
							<failure message="failure message 0" type="failure"> failure value 0 </failure>
						</testcase>
						<testcase classname="cls" name="test2"></testcase>
					</testsuite>
				</testsuites>
				`),
			},
			groups:    nil,
			expFailed: 1,
			expPassed: 1,
			expGroups: nil,
		},
		{
			name: "group with no matches produces empty group",
			rawResults: [][]byte{
				[]byte(`
				<testsuites>
					<testsuite>
						<testcase classname="cls" name="test1">
							<failure message="failure message 0" type="failure"> failure value 0 </failure>
						</testcase>
					</testsuite>
				</testsuites>
				`),
			},
			groups: []GroupConfig{
				{
					Name:     "Informing",
					Selector: "properties/property[@name='lifecycle' and @value='informing']",
				},
			},
			expFailed: 1,
			expPassed: 0,
			expGroups: []struct {
				name    string
				failed  int
				passed  int
				skipped int
			}{
				{name: "Informing", failed: 0, passed: 0, skipped: 0},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			artifacts := make([]api.Artifact, 0)
			for _, rr := range tt.rawResults {
				artifacts = append(artifacts, &FakeArtifact{
					path:      "log.txt",
					content:   rr,
					sizeLimit: 500e6,
				})
			}
			l := Lens{}
			got := l.getJvd(artifacts, tt.groups)
			if len(got.Failed) != tt.expFailed {
				t.Errorf("Failed count = %d, want %d", len(got.Failed), tt.expFailed)
			}
			if len(got.Passed) != tt.expPassed {
				t.Errorf("Passed count = %d, want %d", len(got.Passed), tt.expPassed)
			}
			if tt.expGroups == nil {
				if len(got.Groups) != 0 {
					t.Errorf("Groups count = %d, want 0", len(got.Groups))
				}
			} else {
				if len(got.Groups) != len(tt.expGroups) {
					t.Fatalf("Groups count = %d, want %d", len(got.Groups), len(tt.expGroups))
				}
				for i, eg := range tt.expGroups {
					g := got.Groups[i]
					if g.Name != eg.name {
						t.Errorf("Group[%d].Name = %q, want %q", i, g.Name, eg.name)
					}
					if len(g.Failed) != eg.failed {
						t.Errorf("Group[%d].Failed = %d, want %d", i, len(g.Failed), eg.failed)
					}
					if len(g.Passed) != eg.passed {
						t.Errorf("Group[%d].Passed = %d, want %d", i, len(g.Passed), eg.passed)
					}
					if len(g.Skipped) != eg.skipped {
						t.Errorf("Group[%d].Skipped = %d, want %d", i, len(g.Skipped), eg.skipped)
					}
				}
			}
		})
	}
}

func TestTemplate(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name               string
		input              JVD
		expectedSubstrings []string
	}{
		{
			name: "Both stdout and stderr get rendered when there is one test",
			input: JVD{NumTests: 1, Failed: []TestResult{{
				Junit: []JunitResult{{
					Result: junit.Result{
						Output: new("output"),
						Error:  new("error"),
					},
				}},
			}}},
			expectedSubstrings: []string{
				`<a href="#" class="open-stdout-stderr">open stdout<i class="material-icons" `,
				`<a href="#" class="open-stdout-stderr">open stderr<i class="material-icons"`,
				`output`,
				`error`,
			},
		},
		{
			name: "Both stdout and stderr get rendered when there are multiple tests",
			input: JVD{NumTests: 1, Failed: []TestResult{{
				Junit: []JunitResult{
					{
						Result: junit.Result{
							Output: new("output"),
							Error:  new("error"),
						},
					},
					{
						Result: junit.Result{
							Output: new("output"),
							Error:  new("error"),
						},
					},
				},
			}}},
			expectedSubstrings: []string{
				`<a href="#" class="open-stdout-stderr">open stdout<i class="material-icons" `,
				`<a href="#" class="open-stdout-stderr">open stderr<i class="material-icons"`,
				`<td class="mdl-data-table__cell--non-numeric test-name">Run #0`,
				`<td class="mdl-data-table__cell--non-numeric test-name">Run #1`,
				`output`,
				`error`,
			},
		},
		{
			name: "Both stdout and stderr get rendered for flaky tests",
			input: JVD{NumTests: 1, Flaky: []TestResult{{
				Junit: []JunitResult{{
					Result: junit.Result{
						Output: new("output"),
						Error:  new("error"),
					},
				}},
			}}},
			expectedSubstrings: []string{
				`<a href="#" class="open-stdout-stderr">open stdout<i class="material-icons" `,
				`<a href="#" class="open-stdout-stderr">open stderr<i class="material-icons"`,
				`output`,
				`error`,
			},
		}, {
			name: "Group section shows full breakdown",
			input: JVD{Groups: []GroupResult{{
				Name:      "Informing",
				Collapsed: true,
				NumTests:  2,
				Failed: []TestResult{{
					Junit: []JunitResult{{
						Result: junit.Result{
							Name:      "informing_fail",
							ClassName: "fake_class",
							Failure:   &junit.Failure{Value: "non-blocking failure"},
						},
					}},
				}},
				Passed: []TestResult{{
					Junit: []JunitResult{{
						Result: junit.Result{
							Name:      "informing_pass",
							ClassName: "fake_class",
						},
					}},
				}},
			}}},
			expectedSubstrings: []string{
				`Informing`,
				`1/2 Failed.`,
				`1/2 Passed.`,
				`group-layout`,
				`fake_class: informing_fail`,
				`hidden-tests`,
			},
		},
		{
			name: "Group section expanded when collapsed is false",
			input: JVD{Groups: []GroupResult{{
				Name:      "MyGroup",
				Collapsed: false,
				NumTests:  1,
				Failed: []TestResult{{
					Junit: []JunitResult{{
						Result: junit.Result{
							Name:      "test_1",
							ClassName: "cls",
							Failure:   &junit.Failure{Value: "fail"},
						},
					}},
				}},
			}}},
			expectedSubstrings: []string{
				`1/1 Failed.`,
				`expand_less`,
			},
		},
	}

	tmpl, err := template.ParseFiles("template.html")
	if err != nil {
		t.Fatalf("failed to parse template: %v", err)
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := tmpl.ExecuteTemplate(&buf, "body", tc.input); err != nil {
				t.Fatalf("failed to execute template: %v", err)
			}
			result := buf.String()

			for _, substring := range tc.expectedSubstrings {
				if !strings.Contains(result, substring) {
					t.Errorf("expected to find substring '%s' in rendered template '%s', wasn't the case", substring, result)
				}
			}
		})
	}
}

func TestGetJvdWithSuiteProperties(t *testing.T) {
	const selector = "properties/property[@name='lifecycle' and @value='informing']"
	inherited := GroupConfig{Name: "Inherited", Selector: selector, IncludeSuiteProperties: true}
	ordinary := GroupConfig{Name: "Ordinary", Selector: selector}
	tests := []struct {
		name        string
		xml         string
		groups      []GroupConfig
		wantDefault int
		wantGroups  []int
	}{
		{
			name:   "absent properties",
			xml:    `<testsuite><testcase name="absent"/><testcase name="empty"><properties/></testcase></testsuite>`,
			groups: []GroupConfig{inherited}, wantDefault: 2, wantGroups: []int{0},
		},
		{
			name:   "suite properties require opt in",
			xml:    `<testsuite><properties><property name="lifecycle" value="informing"/></properties><testcase name="test"/></testsuite>`,
			groups: []GroupConfig{ordinary}, wantDefault: 1, wantGroups: []int{0},
		},
		{
			name:   "suite only and testcase override",
			xml:    `<testsuite><properties><property name="lifecycle" value="informing"/></properties><testcase name="inherited"/><testcase name="override"><properties><property name="lifecycle" value="blocking"/></properties></testcase></testsuite>`,
			groups: []GroupConfig{inherited}, wantDefault: 1, wantGroups: []int{1},
		},
		{
			name:   "nested suite override and sibling isolation",
			xml:    `<testsuites><testsuite name="outer"><properties><property name="lifecycle" value="informing"/></properties><testsuite name="near"><properties><property name="lifecycle" value="blocking"/></properties><testcase name="near"/><testsuite name="deep"><testcase name="deep"/></testsuite></testsuite><testsuite name="sibling"><testcase name="sibling"/></testsuite><testcase name="outer"/></testsuite><testsuite name="separate"><testcase name="separate"/></testsuite></testsuites>`,
			groups: []GroupConfig{inherited}, wantDefault: 3, wantGroups: []int{2},
		},
		{
			name:       "nonmatching opted in group does not mutate testcase properties",
			xml:        `<testsuite><properties><property name="lifecycle" value="informing"/></properties><testcase name="test"/></testsuite>`,
			groups:     []GroupConfig{{Name: "No match", Selector: selector + "properties/property[@name='missing']", IncludeSuiteProperties: true}, ordinary, inherited},
			wantGroups: []int{0, 0, 1},
		},
		{
			name:   "ordinary group retains first match priority",
			xml:    `<testsuite><properties><property name="lifecycle" value="blocking"/></properties><testcase name="test"><properties><property name="lifecycle" value="informing"/></properties></testcase></testsuite>`,
			groups: []GroupConfig{ordinary, inherited}, wantGroups: []int{1, 0},
		},
		{
			name:   "opted in group retains first match priority",
			xml:    `<testsuite><testcase name="test"><properties><property name="lifecycle" value="informing"/></properties></testcase></testsuite>`,
			groups: []GroupConfig{inherited, ordinary}, wantGroups: []int{1, 0},
		},
		{
			name:   "repeated tests use first run properties and keep counts",
			xml:    `<testsuite><properties><property name="lifecycle" value="informing"/></properties><testcase name="flaky"><failure/></testcase><testcase name="flaky"><properties><property name="lifecycle" value="blocking"/></properties></testcase><testcase name="skip"><skipped/></testcase><testcase name="fail"><failure/></testcase><testcase name="pass"/></testsuite>`,
			groups: []GroupConfig{inherited}, wantGroups: []int{4},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := (Lens{}).getJvd([]api.Artifact{&FakeArtifact{path: "junit.xml", content: []byte(tc.xml), sizeLimit: 500e6}}, tc.groups)
			if got.NumTests != tc.wantDefault {
				t.Errorf("default count = %d, want %d", got.NumTests, tc.wantDefault)
			}
			wantTotal := tc.wantDefault
			for i, want := range tc.wantGroups {
				wantTotal += want
				if got.Groups[i].NumTests != want {
					t.Errorf("group %d count = %d, want %d", i, got.Groups[i].NumTests, want)
				}
			}
			if got.TotalTests() != wantTotal {
				t.Errorf("total count = %d, want %d", got.TotalTests(), wantTotal)
			}
			if tc.name == "repeated tests use first run properties and keep counts" {
				g := got.Groups[0]
				if len(g.Flaky) != 1 || len(g.Flaky[0].Junit) != 2 || len(g.Passed) != 1 || len(g.Failed) != 1 || len(g.Skipped) != 1 {
					t.Errorf("unexpected repeated test classification: %+v", g)
				}
			}
		})
	}
}

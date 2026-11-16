/*
Copyright 2018 The Kubernetes Authors.

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

package main

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"sigs.k8s.io/prow/pkg/config"
)

func TestDeckPageTemplates(t *testing.T) {
	files, err := filepath.Glob("template/*.html")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) < 2 {
		t.Fatal("expected base template and page templates")
	}
	for _, file := range files {
		name := filepath.Base(file)
		if name == "base.html" {
			continue
		}
		for _, logo := range []string{"", "https://example.com/custom-logo.png"} {
			branding := "default"
			if logo != "" {
				branding = "custom"
			}
			t.Run(name+"/"+branding, func(t *testing.T) {
				cfg := func() *config.Config {
					return &config.Config{ProwConfig: config.ProwConfig{Deck: config.Deck{Branding: &config.Branding{Logo: logo}}}}
				}
				response := httptest.NewRecorder()
				handleSimpleTemplate(options{templateFilesLocation: "template"}, cfg, name, map[string]any{})(response, httptest.NewRequest(http.MethodGet, "/", nil))
				if response.Code != http.StatusOK {
					t.Fatalf("rendering failed: %d: %s", response.Code, response.Body.String())
				}
				body := response.Body.String()
				if logo == "" {
					for _, image := range []string{"logo-light.png?v=", "logo-dark.png?v="} {
						if !strings.Contains(body, image) {
							t.Errorf("default logo variant %q missing", image)
						}
					}
				} else {
					if !strings.Contains(body, logo+"?v=") {
						t.Error("custom branding logo missing")
					}
					if strings.Contains(body, "/static/logo-") {
						t.Error("default logo must not override custom branding")
					}
				}
				isIndex := name == "index.html"
				if got := strings.Contains(body, `id="theme-toggle"`); got != isIndex {
					t.Errorf("theme toggle present = %t, want %t", got, isIndex)
				}
				if got := strings.Contains(body, "var prowThemeStorage"); got != isIndex {
					t.Errorf("theme bootstrap present = %t, want %t", got, isIndex)
				}
				if name == "spyglass.html" && !strings.Contains(body, `<body id="spyglass"`) {
					t.Error("Spyglass must retain its static dark theme selector")
				}
			})
		}
	}
}

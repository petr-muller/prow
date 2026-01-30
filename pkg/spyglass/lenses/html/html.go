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

package html

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"text/template"

	"golang.org/x/net/html"

	"github.com/sirupsen/logrus"

	"sigs.k8s.io/prow/pkg/config"
	"sigs.k8s.io/prow/pkg/spyglass/api"
	"sigs.k8s.io/prow/pkg/spyglass/lenses"
)

// maxMetadataBytes is the maximum number of bytes to read when extracting metadata.
// 64KB should be enough to capture the <head> section of most HTML files.
const maxMetadataBytes = 64 * 1024

// callbackRequest represents the JSON request format for callbacks
type callbackRequest struct {
	Type  string `json:"type"`  // "metadata" or "content"
	Index int    `json:"index"` // artifact index
}

// metadataResponse represents the JSON response for metadata requests
type metadataResponse struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
}

func init() {
	lenses.RegisterLens(Lens{})
}

type Lens struct{}

type document struct {
	Filename string
	ID       string
	Title    string
	Content  string
	Index    int // Index in the artifacts array for lazy loading
}

// Config returns the lens's configuration.
func (lens Lens) Config() lenses.LensConfig {
	return lenses.LensConfig{
		Name:      "html",
		Title:     "HTML",
		Priority:  3,
		HideTitle: true,
	}
}

// Header renders the content of <head> from template.html.
func (lens Lens) Header(artifacts []api.Artifact, resourceDir string, config json.RawMessage, spyglassConfig config.Spyglass) string {
	t, err := template.ParseFiles(filepath.Join(resourceDir, "template.html"))
	if err != nil {
		return fmt.Sprintf("<!-- FAILED LOADING HEADER: %v -->", err)
	}
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "header", nil); err != nil {
		return fmt.Sprintf("<!-- FAILED EXECUTING HEADER TEMPLATE: %v -->", err)
	}
	return buf.String()
}

// Callback handles lazy loading of individual HTML artifacts.
// Expects data to be JSON with format: {"type": "metadata"|"content", "index": N}
// - "metadata": Returns JSON with title and description extracted from HTML head
// - "content": Returns full HTML content for iframe display
func (lens Lens) Callback(artifacts []api.Artifact, resourceDir string, data string, config json.RawMessage, spyglassConfig config.Spyglass) string {
	if data == "" {
		return ""
	}

	var req callbackRequest
	if err := json.Unmarshal([]byte(data), &req); err != nil {
		logrus.WithError(err).Error("Failed to parse callback request JSON")
		return ""
	}

	if req.Index < 0 || req.Index >= len(artifacts) {
		logrus.Errorf("Invalid artifact index %d, only %d artifacts available", req.Index, len(artifacts))
		return ""
	}

	artifact := artifacts[req.Index]
	name := filepath.Base(artifact.CanonicalLink())

	switch req.Type {
	case "metadata":
		return lens.handleMetadataRequest(artifact, name)
	case "content":
		return lens.handleContentRequest(artifact, name, req.Index)
	default:
		logrus.Errorf("Unknown callback request type: %s", req.Type)
		return ""
	}
}

// handleMetadataRequest extracts title and description from the HTML head section
// without reading the entire file (limited to first 64KB)
func (lens Lens) handleMetadataRequest(artifact api.Artifact, name string) string {
	content, err := readAtMostBytes(artifact, maxMetadataBytes)
	if err != nil {
		logrus.WithError(err).WithField("artifact_url", artifact.CanonicalLink()).Warn("failed to read metadata")
		return ""
	}

	title, description := extractMetadataOnly(content, name)

	resp := metadataResponse{
		Title:       title,
		Description: description,
	}

	jsonBytes, err := json.Marshal(resp)
	if err != nil {
		logrus.WithError(err).Error("Failed to marshal metadata response")
		return ""
	}

	return string(jsonBytes)
}

// handleContentRequest returns the full HTML content for iframe display
func (lens Lens) handleContentRequest(artifact api.Artifact, name string, index int) string {
	content, err := artifact.ReadAll()
	if err != nil {
		logrus.WithError(err).WithField("artifact_url", artifact.CanonicalLink()).Warn("failed to read content")
		return ""
	}

	id := fmt.Sprintf("%s-%d", name, index)

	// For callback (lazy loading via AJAX), we inject the height notifier but don't escape quotes
	// since the content will be set via JavaScript, not embedded in HTML attributes
	return injectHeightNotifier(string(content), id)
}

// readAtMostBytes reads up to n bytes from the artifact
func readAtMostBytes(artifact api.Artifact, n int64) ([]byte, error) {
	size, err := artifact.Size()
	if err != nil {
		return nil, err
	}

	readSize := size
	if readSize > n {
		readSize = n
	}

	buf := make([]byte, readSize)
	_, err = artifact.ReadAt(buf, 0)
	if err != nil && err != io.EOF {
		return nil, err
	}

	return buf, nil
}

// extractMetadataOnly parses HTML to extract just the title and meta description
// without processing the entire document
func extractMetadataOnly(content []byte, defaultTitle string) (title string, description string) {
	title = defaultTitle

	token := html.NewTokenizer(bytes.NewReader(content))
	isTitle := false
	inHead := false

	for {
		tt := token.Next()
		switch tt {
		case html.ErrorToken:
			// End of document or error - return what we have
			return title, description
		case html.StartTagToken, html.SelfClosingTagToken:
			t := token.Token()
			switch t.Data {
			case "head":
				inHead = true
			case "body":
				// Once we hit body, we're done with head section
				return title, description
			case "title":
				isTitle = true
			case "meta":
				if inHead || true { // Check meta tags even outside explicit head
					content := ""
					isDescription := false
					for _, attr := range t.Attr {
						if attr.Key == "name" && attr.Val == "description" {
							isDescription = true
						} else if attr.Key == "content" {
							content = attr.Val
						}
					}
					if isDescription && content != "" {
						description = content
					}
				}
			}
		case html.EndTagToken:
			t := token.Token()
			if t.Data == "head" {
				// End of head section
				return title, description
			}
		case html.TextToken:
			if isTitle {
				isTitle = false
				t := token.Token()
				if t.Data != "" {
					title = strings.TrimSpace(t.Data)
				}
			}
		}
	}
}

// Body renders the <body>
// For lazy loading, we don't read any content - just use filenames as titles.
func (lens Lens) Body(artifacts []api.Artifact, resourceDir string, data string, config json.RawMessage, spyglassConfig config.Spyglass) string {
	if len(artifacts) == 0 {
		logrus.Error("html Body() called with no artifacts, which should never happen.")
		return "Why am I here? There is no html file"
	}

	documents := make([]document, 0)
	for i, artifact := range artifacts {
		name := filepath.Base(artifact.CanonicalLink())
		// For lazy loading, don't read content - just use filename
		doc := document{
			Filename: name,
			ID:       fmt.Sprintf("%s-%d", name, i),
			Title:    name, // Use filename as title initially
			Content:  "",   // Empty - will be fetched via callback
			Index:    i,
		}
		documents = append(documents, doc)
	}

	template, err := template.ParseFiles(filepath.Join(resourceDir, "template.html"))
	if err != nil {
		logrus.WithError(err).Error("Error executing template.")
		return fmt.Sprintf("Failed to load template file: %v", err)
	}

	buf := &bytes.Buffer{}
	if err := template.ExecuteTemplate(buf, "body", documents); err != nil {
		return fmt.Sprintf("failed to execute template: %v", err)
	}
	return buf.String()
}

// extractDocumentDetails parses the HTML to extract the title and
// meta description tag, if present.
func extractDocumentDetails(name string, id int, content []byte) document {
	doc := document{
		Filename: name,
		ID:       fmt.Sprintf("%s-%d", name, id),
		Title:    name,
		Content:  string(content),
	}

	description := ""
	token := html.NewTokenizer(bytes.NewReader(content))
	isTitle := false
	for {
		switch token.Next() {
		case html.ErrorToken:
			doc.Content = injectHeightNotifier(doc.Content, doc.ID)
			// Escape double quotes as we are going to put this into an iframes srcdoc attribute. We can not reference the
			// src directly because we have to inject the height notifier.
			// Ref: https://html.spec.whatwg.org/multipage/iframe-embed-object.html#attr-iframe-srcdoc
			doc.Content = strings.ReplaceAll(doc.Content, `"`, `&quot;`)

			if description != "" {
				doc.Title = doc.Title + fmt.Sprintf(` <abbr class="icon material-icons" title="%s">info</abbr>`, description)
			}

			return doc
		case html.StartTagToken, html.SelfClosingTagToken:
			tt := token.Token()
			switch tt.Data {
			case "title":
				isTitle = true
			case "meta":
				content := ""
				isDescription := false
				for _, attr := range tt.Attr {
					if attr.Key == "name" && attr.Val == "description" {
						isDescription = true
					} else if attr.Key == "content" {
						content = attr.Val
					}
				}
				if isDescription {
					description = content
				}
			}
		case html.TextToken:
			if isTitle {
				isTitle = false
				tt := token.Token()
				if tt.Data != "" {
					doc.Title = tt.Data
				}
			}
		}
	}
}

// injectHeightNotifier injects a small javascript snippet that will tell the iframe container about the height
// of the iframe. Iframe height can only be set as an absolute value and CORS doesn't allow the container to
// query the iframe.
func injectHeightNotifier(content string, id string) string {
	return `<div id="wrapper">` + content + fmt.Sprintf(`</div><script type="text/javascript">
window.addEventListener("load", function(){
    if(window.self === window.top) return; // if w.self === w.top, we are not in an iframe
    send_height_to_parent_function = function(){
        var height = document.getElementById("wrapper").offsetHeight;
        parent.postMessage({"height" : height , "id": "%s"}, "*");
    }
    send_height_to_parent_function(); //whenever the page is loaded
    window.addEventListener("resize", send_height_to_parent_function); // whenever the page is resized
    var observer = new MutationObserver(send_height_to_parent_function);           // whenever DOM changes PT1
    var config = { attributes: true, childList: true, characterData: true, subtree:true}; // PT2
    observer.observe(window.document, config);                                            // PT3
});
</script>`, id)
}

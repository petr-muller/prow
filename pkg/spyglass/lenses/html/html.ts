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

interface MetadataResponse {
  title: string;
  description?: string;
}

const fetchAllMetadata = (): void => {
  const headers = document.querySelectorAll<HTMLTableRowElement>('tr.section-expander[data-artifact-index]');
  for (const header of Array.from(headers)) {
    const index = header.dataset.artifactIndex;
    if (!index) continue;

    const titleSpan = header.querySelector('.artifact-title') as HTMLElement;
    const spinner = header.querySelector('.metadata-spinner') as HTMLElement;

    if (!titleSpan || !spinner) continue;

    spyglass.request(JSON.stringify({type: "metadata", index: parseInt(index, 10)}))
      .then((response: string) => {
        const data: MetadataResponse = JSON.parse(response);
        if (data.title) {
          let displayTitle = data.title;
          if (data.description) {
            displayTitle += ` <abbr class="icon material-icons" title="${data.description}">info</abbr>`;
          }
          titleSpan.innerHTML = displayTitle;
        }
        spinner.style.display = 'none';
      })
      .catch(() => {
        spinner.style.display = 'none'; // Keep filename on error
      });
  }
};

const addSectionExpanders = (): void => {
  const expanders = document.querySelectorAll<HTMLTableRowElement>('tr.section-expander');
  for (const expander of Array.from(expanders)) {
    expander.onclick = () => {
      const nextRow = expander.nextElementSibling as HTMLTableRowElement;
      const icon = expander.querySelector('i')!;
      if (nextRow.classList.contains('hidden-data')) {
        // Expanding: lazy-load the HTML content from server if not already loaded
        const iframe = nextRow.querySelector('iframe') as HTMLIFrameElement;
        const spinnerContainer = nextRow.querySelector('.html-loading-container') as HTMLElement;
        if (iframe && !iframe.srcdoc) {
          const index = nextRow.dataset.index;
          if (index) {
            // Show spinner while loading
            if (spinnerContainer) {
              spinnerContainer.style.display = 'block';
              iframe.style.display = 'none';
            }
            // Fetch content from server via callback using JSON format
            spyglass.request(JSON.stringify({type: "content", index: parseInt(index, 10)})).then((content: string) => {
              iframe.srcdoc = content;
              // Hide spinner and show iframe
              if (spinnerContainer) {
                spinnerContainer.style.display = 'none';
                iframe.style.display = 'block';
              }
              spyglass.contentUpdated();
            }).catch((error: Error) => {
              console.error('Failed to load HTML artifact:', error);
              iframe.srcdoc = '<html><body><h3>Failed to load content</h3></body></html>';
              // Hide spinner and show error
              if (spinnerContainer) {
                spinnerContainer.style.display = 'none';
                iframe.style.display = 'block';
              }
            });
          }
        }
        nextRow.classList.remove('hidden-data');
        icon.innerText = 'expand_less';
      } else {
        nextRow.classList.add('hidden-data');
        icon.innerText = 'expand_more';
      }
      spyglass.contentUpdated();
    };
  }
};

const resizeIframe = (e: MessageEvent): void => {
  const iFrame = document.getElementById(String(e.data.id)) as HTMLIFrameElement;
  if (!iFrame) {
    return;
  }
  if (iFrame.contentWindow !== e.source) {
    return;
  }
  const height = `${e.data.height  }px`;
  iFrame.height = height;
  iFrame.style.height = height;
  spyglass.contentUpdated();
};

window.addEventListener('DOMContentLoaded', () => {
  addSectionExpanders();
  fetchAllMetadata();
});
window.addEventListener('message', resizeIframe);

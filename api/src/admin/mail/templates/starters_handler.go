package templates

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/a-digi/coco-iam/config"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// AdminMailTemplatesStartersHandler serves
// GET /api/v1/admin/mail/templates/starters.
// Returns the built-in starter catalog — a list of template skeletons the
// frontend can use to pre-fill the Create-Template form. Starters are
// shipped as embedded files under api/config/mail/templates/starters/;
// this handler reads the manifest + each starter's bodies and merges them
// into a single response.
type AdminMailTemplatesStartersHandler struct{}

const startersDir = "mail/templates/starters"
const manifestFile = "mail/templates/starters/manifest.json"

type starterToken struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Example     string `json:"example,omitempty"`
}

type starterManifestEntry struct {
	Name        string         `json:"name"`
	DisplayName string         `json:"display_name"`
	Description string         `json:"description"`
	Tokens      []starterToken `json:"tokens"`
}

type starterManifest struct {
	Starters []starterManifestEntry `json:"starters"`
}

type starterResponse struct {
	Name        string         `json:"name"`
	DisplayName string         `json:"display_name"`
	Description string         `json:"description"`
	Subject     string         `json:"subject"`
	TextBody    string         `json:"text_body"`
	HTMLBody    string         `json:"html_body"`
	Tokens      []starterToken `json:"tokens"`
}

func (h *AdminMailTemplatesStartersHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()

	manifestBytes, err := fs.ReadFile(config.ConfigFS, manifestFile)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to read starter manifest: "+err.Error())
		return
	}
	var manifest starterManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to parse starter manifest: "+err.Error())
		return
	}

	out := make([]starterResponse, 0, len(manifest.Starters))
	for _, entry := range manifest.Starters {
		textRaw, _ := fs.ReadFile(config.ConfigFS, path.Join(startersDir, entry.Name+".txt"))
		htmlRaw, _ := fs.ReadFile(config.ConfigFS, path.Join(startersDir, entry.Name+".html"))
		if len(textRaw) == 0 && len(htmlRaw) == 0 {
			// Skip entries with no body — surfaces stale manifest entries
			// loudly in logs rather than silently broken starters in the UI.
			continue
		}

		subject := ""
		textSubject, textBody := splitSubject(string(textRaw))
		htmlSubject, htmlBody := splitSubject(string(htmlRaw))
		switch {
		case htmlSubject != "":
			subject = htmlSubject
		case textSubject != "":
			subject = textSubject
		}

		out = append(out, starterResponse{
			Name:        entry.Name,
			DisplayName: entry.DisplayName,
			Description: entry.Description,
			Subject:     subject,
			TextBody:    textBody,
			HTMLBody:    htmlBody,
			Tokens:      entry.Tokens,
		})
	}

	response.SuccessResponse(w, http.StatusOK, out)
}

// splitSubject strips an optional leading `Subject: <text>` line from a
// template body. Kept local so the starters endpoint doesn't depend on the
// renderer package.
func splitSubject(content string) (string, string) {
	if content == "" {
		return "", ""
	}
	newline := strings.IndexByte(content, '\n')
	if newline < 0 {
		return "", content
	}
	first := strings.TrimRight(content[:newline], "\r")
	if !strings.HasPrefix(first, "Subject:") {
		return "", content
	}
	subject := strings.TrimSpace(strings.TrimPrefix(first, "Subject:"))
	body := strings.TrimLeft(content[newline+1:], "\n")
	return subject, body
}


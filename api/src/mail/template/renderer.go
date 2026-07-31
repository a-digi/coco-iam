// Package template renders mail templates.
//
// Lookup order on every Render call:
//  1. If a Repository is configured, look up `name` in mail.db.mail_templates
//     where is_active = TRUE.
//  2. Otherwise fall back to the embedded FS pair under
//     `config/mail/templates/<name>.{html,txt}`.
//
// DB rows carry their own Subject column; file templates keep the legacy
// `Subject: <text>` first-line convention (still honoured).
package template

import (
	"bytes"
	"errors"
	"fmt"
	htmltemplate "html/template"
	"io/fs"
	"path"
	"strings"
	"sync"
	texttemplate "text/template"
)

const templatesDir = "mail/templates"

// Renderer loads templates on demand. Safe for concurrent use.
type Renderer struct {
	fs   fs.FS
	repo *Repository

	mu        sync.RWMutex
	fileCache map[string]*cachedTemplate
	dbCache   map[string]*cachedDBTemplate
}

type cachedTemplate struct {
	htmlSubject string
	textSubject string
	html        *htmltemplate.Template
	text        *texttemplate.Template
}

type cachedDBTemplate struct {
	updatedAt string
	subject   string
	html      *htmltemplate.Template
	text      *texttemplate.Template
}

// Option is a functional option for New.
type Option func(*Renderer)

// WithRepository enables the DB-first lookup path. Pass nil to disable
// (renderer then uses the embedded FS only).
func WithRepository(repo *Repository) Option {
	return func(r *Renderer) { r.repo = repo }
}

// New constructs a Renderer reading from the given filesystem. A nil fs
// disables file-based lookups (only DB templates via WithRepository will
// work).
func New(embed fs.FS, opts ...Option) (*Renderer, error) {
	r := &Renderer{
		fileCache: map[string]*cachedTemplate{},
		dbCache:   map[string]*cachedDBTemplate{},
	}
	for _, opt := range opts {
		opt(r)
	}
	if embed != nil {
		if _, err := fs.Stat(embed, templatesDir); err != nil {
			return nil, fmt.Errorf("mail templates: %w", err)
		}
		r.fs = embed
	}
	return r, nil
}

// Render compiles (or reuses) the named template and applies data. Returns
// (subject, textBody, htmlBody, error). Either body may be empty if only
// one was defined.
func (r *Renderer) Render(name string, data map[string]interface{}) (string, string, string, error) {
	// 1. DB path first.
	if r.repo != nil {
		subject, text, html, ok, err := r.renderFromDB(name, data)
		if err != nil {
			return "", "", "", err
		}
		if ok {
			return subject, text, html, nil
		}
	}

	// 2. Embedded fallback.
	t, err := r.loadFile(name)
	if err != nil {
		return "", "", "", err
	}
	var textBuf, htmlBuf bytes.Buffer
	if t.text != nil {
		if err := t.text.Execute(&textBuf, data); err != nil {
			return "", "", "", fmt.Errorf("mail: render text %q: %w", name, err)
		}
	}
	if t.html != nil {
		if err := t.html.Execute(&htmlBuf, data); err != nil {
			return "", "", "", fmt.Errorf("mail: render html %q: %w", name, err)
		}
	}
	subject := t.htmlSubject
	if subject == "" {
		subject = t.textSubject
	}
	// Subject lines are templated just like bodies — otherwise tokens
	// like `{{ .WebsiteTitle }}` in `Subject: Reset your {{ .WebsiteTitle }} password`
	// leak through literally in the user's inbox.
	return renderSubject(name, subject, data), textBuf.String(), htmlBuf.String(), nil
}

// RenderStrings compiles and executes a subject/text/html triple as
// templates against data, without any DB or embedded-FS lookup — a
// pure function callers with their own already-fetched template row
// (e.g. an organization- or application-scoped one) can use directly.
// Extracted from parseDBRow/renderFromDB's own compile+execute steps so
// there is exactly one place that knows how a mail template's raw
// fields become rendered output. Subject may be empty (falls back to
// whichever body's own leading "Subject: " line was present, if any —
// none in this path, since raw callers pass an already-split subject).
func RenderStrings(name, subject, textBody, htmlBody string, data map[string]interface{}) (string, string, string, error) {
	if textBody == "" && htmlBody == "" {
		return "", "", "", fmt.Errorf("mail: template %q has empty body", name)
	}
	var textBuf, htmlBuf bytes.Buffer
	if textBody != "" {
		tpl, err := texttemplate.New(name + ".txt").Parse(textBody)
		if err != nil {
			return "", "", "", fmt.Errorf("mail: parse text %q: %w", name, err)
		}
		if err := tpl.Execute(&textBuf, data); err != nil {
			return "", "", "", fmt.Errorf("mail: render text %q: %w", name, err)
		}
	}
	if htmlBody != "" {
		tpl, err := htmltemplate.New(name + ".html").Parse(htmlBody)
		if err != nil {
			return "", "", "", fmt.Errorf("mail: parse html %q: %w", name, err)
		}
		if err := tpl.Execute(&htmlBuf, data); err != nil {
			return "", "", "", fmt.Errorf("mail: render html %q: %w", name, err)
		}
	}
	return renderSubject(name, subject, data), textBuf.String(), htmlBuf.String(), nil
}

func (r *Renderer) renderFromDB(name string, data map[string]interface{}) (string, string, string, bool, error) {
	row, err := r.repo.GetByName(name)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return "", "", "", false, nil
		}
		return "", "", "", false, fmt.Errorf("mail: db template %q: %w", name, err)
	}
	if !row.IsActive {
		return "", "", "", false, nil
	}

	// Cache keyed by (name, updated_at). When the admin edits the row we
	// see a new updated_at and reparse.
	r.mu.RLock()
	cached, ok := r.dbCache[name]
	r.mu.RUnlock()
	if !ok || cached.updatedAt != row.UpdatedAt {
		parsed, perr := parseDBRow(row)
		if perr != nil {
			return "", "", "", false, perr
		}
		r.mu.Lock()
		r.dbCache[name] = parsed
		r.mu.Unlock()
		cached = parsed
	}

	var textBuf, htmlBuf bytes.Buffer
	if cached.text != nil {
		if err := cached.text.Execute(&textBuf, data); err != nil {
			return "", "", "", false, fmt.Errorf("mail: render text %q: %w", name, err)
		}
	}
	if cached.html != nil {
		if err := cached.html.Execute(&htmlBuf, data); err != nil {
			return "", "", "", false, fmt.Errorf("mail: render html %q: %w", name, err)
		}
	}
	return renderSubject(name, cached.subject, data), textBuf.String(), htmlBuf.String(), true, nil
}

// renderSubject compiles and executes a subject line as a text
// template against the same data bag the body got. On any compile or
// execute error, it falls back to the raw string — a broken subject
// shouldn't block the email from going out.
func renderSubject(name, subject string, data map[string]interface{}) string {
	if subject == "" || !strings.Contains(subject, "{{") {
		return subject
	}
	tpl, err := texttemplate.New(name + ".subject").Parse(subject)
	if err != nil {
		return subject
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return subject
	}
	return buf.String()
}

func parseDBRow(row *Template) (*cachedDBTemplate, error) {
	out := &cachedDBTemplate{updatedAt: row.UpdatedAt, subject: row.Subject}
	if row.TextBody == "" && row.HTMLBody == "" {
		return nil, fmt.Errorf("mail: template %q has empty body", row.Name)
	}
	if row.TextBody != "" {
		subject, body := splitSubject(row.TextBody)
		tpl, err := texttemplate.New(row.Name + ".txt").Parse(body)
		if err != nil {
			return nil, fmt.Errorf("mail: parse text %q: %w", row.Name, err)
		}
		out.text = tpl
		if out.subject == "" {
			out.subject = subject
		}
	}
	if row.HTMLBody != "" {
		subject, body := splitSubject(row.HTMLBody)
		tpl, err := htmltemplate.New(row.Name + ".html").Parse(body)
		if err != nil {
			return nil, fmt.Errorf("mail: parse html %q: %w", row.Name, err)
		}
		out.html = tpl
		if out.subject == "" {
			out.subject = subject
		}
	}
	return out, nil
}

func (r *Renderer) loadFile(name string) (*cachedTemplate, error) {
	r.mu.RLock()
	if t, ok := r.fileCache[name]; ok {
		r.mu.RUnlock()
		return t, nil
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()
	if t, ok := r.fileCache[name]; ok {
		return t, nil
	}

	if r.fs == nil {
		return nil, fmt.Errorf("mail: template %q not found", name)
	}

	t := &cachedTemplate{}

	htmlBytes, htmlErr := fs.ReadFile(r.fs, path.Join(templatesDir, name+".html"))
	textBytes, textErr := fs.ReadFile(r.fs, path.Join(templatesDir, name+".txt"))
	if htmlErr != nil && textErr != nil {
		return nil, fmt.Errorf("mail: template %q not found (need %s.html or %s.txt)", name, name, name)
	}

	if htmlErr == nil {
		subject, body := splitSubject(string(htmlBytes))
		tpl, perr := htmltemplate.New(name + ".html").Parse(body)
		if perr != nil {
			return nil, fmt.Errorf("mail: parse html %q: %w", name, perr)
		}
		t.htmlSubject = subject
		t.html = tpl
	}

	if textErr == nil {
		subject, body := splitSubject(string(textBytes))
		tpl, perr := texttemplate.New(name + ".txt").Parse(body)
		if perr != nil {
			return nil, fmt.Errorf("mail: parse text %q: %w", name, perr)
		}
		t.textSubject = subject
		t.text = tpl
	}

	r.fileCache[name] = t
	return t, nil
}

// splitSubject returns (subject, rest). If the file's first line starts with
// `Subject: `, it's consumed — otherwise the subject is "" and the body is
// returned unchanged.
func splitSubject(content string) (string, string) {
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

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLegalContactEmailsOptOutOfCloudflareObfuscation(t *testing.T) {
	a := args{LegalName: "Example", LegalEmail: "contact@example.test", LegalAddressLines: []string{"Example Street 1"}}
	dir := t.TempDir()
	if err := syncLegalPages(a, dir); err != nil {
		t.Fatal(err)
	}
	want := `<!--email_off--><a href="mailto:contact@example.test">contact@example.test</a><!--/email_off-->`
	for _, name := range []string{"impressum.html", "datenschutz.html", "korrektur.html"} {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				t.Fatal(err)
			}
			page := string(data)
			if !strings.Contains(page, want) {
				t.Fatal("contact email is not protected from Cloudflare rewriting")
			}
			if strings.Contains(strings.ReplaceAll(page, want, ""), "mailto:") {
				t.Fatal("a contact mailto link lacks the opt-out markers")
			}
			if strings.Contains(page, "<script") {
				t.Fatal("legal contact must not depend on JavaScript")
			}
		})
	}
}

func TestLegalContactEmailRemainsEscaped(t *testing.T) {
	a := args{LegalEmail: `bad"><script>alert(1)</script>&@example.test`}
	for _, render := range []func(args) string{impressumPage, datenschutzPage, korrekturPage} {
		page := render(a)
		if strings.Contains(page, `<script>alert(1)</script>`) || strings.Contains(page, a.LegalEmail) {
			t.Fatal("contact email can inject markup")
		}
		if !strings.Contains(page, "&lt;script&gt;") || !strings.Contains(page, "&amp;@example.test") {
			t.Fatal("contact email was not safely escaped")
		}
	}
}

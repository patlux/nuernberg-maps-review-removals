package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func normalizedSiteURL(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "/"
	}
	return strings.TrimRight(value, "/") + "/"
}

// Publish only the dashboard and configured information pages. Research exports,
// old charts and source data must never enter the artifact through a directory glob.
func buildSite(a args) error {
	out := strings.TrimSpace(a.SiteOutput)
	if out == "" {
		out = "public"
	}
	root, err := filepath.Abs(out)
	if err != nil {
		return err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	if within(root, cwd) {
		return fmt.Errorf("unsafe site output directory: contains working directory")
	}
	if info, err := os.Lstat(out); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("site output must be a directory, not a symlink")
		}
		realRoot, err := filepath.EvalSymlinks(root)
		if err != nil {
			return err
		}
		root = realRoot
		realCWD, err := filepath.EvalSymlinks(cwd)
		if err != nil {
			return err
		}
		if within(root, realCWD) {
			return fmt.Errorf("unsafe site output directory: contains working directory")
		}
		for _, name := range []string{".git", "go.mod", "config.toml"} {
			if _, err := os.Lstat(filepath.Join(out, name)); err == nil {
				return fmt.Errorf("site output contains project files")
			} else if !os.IsNotExist(err) {
				return err
			}
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	for _, source := range []string{a.Output, a.Input} {
		if source == "" {
			continue
		}
		absolute, err := filepath.Abs(source)
		if err != nil {
			return err
		}
		if resolved, err := filepath.EvalSymlinks(absolute); err == nil {
			absolute = resolved
		}
		if within(root, absolute) {
			return fmt.Errorf("site output must not contain source files")
		}
	}
	// Validate sources before replacing an existing generated artifact.
	files := map[string]string{"index.html": a.Output}
	if legalPagesEnabled(a) {
		for _, name := range legalPageFiles {
			files[name] = filepath.Join(filepath.Dir(a.Output), name)
		}
	}
	for _, src := range files {
		if info, err := os.Stat(src); err != nil {
			return err
		} else if !info.Mode().IsRegular() {
			return fmt.Errorf("site source is not a regular file")
		}
	}
	if err := os.RemoveAll(out); err != nil {
		return err
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(out, ".nojekyll"), nil, 0o644); err != nil {
		return err
	}
	if strings.TrimSpace(a.SiteDomain) != "" {
		if err := os.WriteFile(filepath.Join(out, "CNAME"), []byte(strings.TrimSpace(a.SiteDomain)+"\n"), 0o644); err != nil {
			return err
		}
	}
	for name, src := range files {
		if err := copyFile(src, filepath.Join(out, name)); err != nil {
			return err
		}
	}
	return writeSiteMeta(a, out)
}

func within(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func writeSiteMeta(a args, out string) error {
	siteURL := normalizedSiteURL(a.SiteURL)
	if siteURL == "/" {
		return nil
	}
	if err := os.WriteFile(filepath.Join(out, "robots.txt"), []byte(fmt.Sprintf("User-agent: *\nAllow: /\nSitemap: %ssitemap.xml\n", siteURL)), 0o644); err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<urlset xmlns=\"http://www.sitemaps.org/schemas/sitemap/0.9\">\n")
	// A build timestamp is not evidence that the historical data was refreshed.
	fmt.Fprintf(&b, "  <url><loc>%s</loc></url>\n", esc(siteURL))
	for _, page := range legalPageFiles {
		if _, err := os.Stat(filepath.Join(out, page)); err == nil {
			fmt.Fprintf(&b, "  <url><loc>%s%s</loc></url>\n", esc(siteURL), page)
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	b.WriteString("</urlset>\n")
	return os.WriteFile(filepath.Join(out, "sitemap.xml"), []byte(b.String()), 0o644)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

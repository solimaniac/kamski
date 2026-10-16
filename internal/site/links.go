package site

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var hrefPattern = regexp.MustCompile(`(?:href|src)="([^"]+)"`)

func validateLinks(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".html" {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return err
		}
		for _, m := range hrefPattern.FindAllSubmatch(b, -1) {
			href := string(m[1])
			if strings.Contains(href, ":") || strings.HasPrefix(href, "#") {
				continue
			}
			target := strings.Split(href, "#")[0]
			if _, err = os.Stat(filepath.Join(dir, target)); err != nil {
				return fmt.Errorf("broken local link %q in %s", href, entry.Name())
			}
		}
	}
	return nil
}

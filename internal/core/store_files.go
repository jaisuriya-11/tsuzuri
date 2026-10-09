package core

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// Dirs returns every non-hidden directory in the workspace as a
// slash-separated path relative to the root, sorted case-insensitively.
// The root itself is represented by "".
func (s *Store) Dirs() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	dirs := []string{""}
	_ = filepath.WalkDir(s.root, func(p string, d os.DirEntry, err error) error {
		if err != nil || !d.IsDir() || p == s.root {
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			return filepath.SkipDir
		}
		rel, relErr := filepath.Rel(s.root, p)
		if relErr != nil {
			return nil
		}
		dirs = append(dirs, filepath.ToSlash(rel))
		return nil
	})
	sort.Slice(dirs, func(i, j int) bool { return strings.ToLower(dirs[i]) < strings.ToLower(dirs[j]) })
	return dirs
}

// ChildDir returns the directory (relative to the root) where new children of
// parentID live: the root, a plain folder, or a page's sidecar folder. Unknown
// parents resolve to the root.
func (s *Store) ChildDir(parentID string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	dir, err := s.childDirFor(parentID)
	if err != nil {
		return ""
	}
	return dir
}

// NormalizeNotePath cleans a user-supplied relative note path and forces the
// ".md" extension (Tsuzuri only ever writes Markdown). It rejects absolute
// paths, hidden segments and anything that would escape the workspace.
func NormalizeNotePath(dir, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("file name is empty")
	}
	if strings.ContainsAny(name, invalidNameChars+"\x00") {
		return "", errors.New(`file name cannot contain / \ : * ? " < > |`)
	}
	if reservedName(name) {
		return "", errors.New(name + " is a reserved name on Windows")
	}
	name = strings.TrimRight(name, ". ")
	if !strings.HasSuffix(strings.ToLower(name), mdExt) {
		name += mdExt
	} else {
		name = name[:len(name)-len(mdExt)] + mdExt
	}
	if strings.TrimSuffix(name, mdExt) == "" {
		return "", errors.New("file name is empty")
	}

	dir = strings.Trim(strings.TrimSpace(filepath.ToSlash(dir)), "/")
	rel := path.Clean(path.Join(dir, name))
	if path.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, "../") {
		return "", errors.New("location must be inside the workspace")
	}
	for _, seg := range strings.Split(rel, "/") {
		if strings.HasPrefix(seg, ".") {
			return "", errors.New("hidden files and folders are not allowed")
		}
	}
	return rel, nil
}

// SaveAs writes content to a new note at dir/name (".md" is added if
// missing), creating any missing folders. It refuses to overwrite an
// existing file.
func (s *Store) SaveAs(dir, name, content string) (Page, error) {
	rel, err := NormalizeNotePath(dir, name)
	if err != nil {
		return Page{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	abs := s.idToAbs(rel)
	if fileExists(abs) {
		return Page{}, fmt.Errorf("%s already exists", rel)
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0755); err != nil {
		return Page{}, fmt.Errorf("failed to create folder: %w", err)
	}
	if err := os.WriteFile(abs, []byte(content), 0644); err != nil {
		return Page{}, fmt.Errorf("failed to save page: %w", err)
	}
	return Page{
		ID:        rel,
		Title:     strings.TrimSuffix(path.Base(rel), mdExt),
		Content:   content,
		ParentID:  parentIDFor(s.root, rel),
		UpdatedAt: modTime(abs),
	}, nil
}

// Rename changes a page's or folder's name on disk without touching its
// content (Update would overwrite the file with p.Content).
func (s *Store) Rename(id, newTitle string) (Page, error) {
	p, err := s.Get(id)
	if err != nil {
		return Page{}, err
	}
	p.Title = strings.TrimSuffix(strings.TrimSpace(newTitle), mdExt)
	return s.Update(p)
}

// AttachFile makes src available to a note living in noteDir (relative to
// the root) and returns the link target to write in the note, relative to
// noteDir. Files already inside the workspace are linked in place; anything
// else is copied into noteDir/assets/ (renamed if the name is taken).
func (s *Store) AttachFile(noteDir, src string) (string, error) {
	absSrc, err := filepath.Abs(src)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(absSrc)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("%s is a folder", filepath.Base(absSrc))
	}
	noteAbs := s.idToAbs(noteDir)

	if rel, err := filepath.Rel(s.root, absSrc); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		link, err := filepath.Rel(noteAbs, absSrc)
		if err != nil {
			return "", err
		}
		return filepath.ToSlash(link), nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	assets := filepath.Join(noteAbs, "assets")
	if err := os.MkdirAll(assets, 0o755); err != nil {
		return "", fmt.Errorf("failed to create assets folder: %w", err)
	}
	ext := filepath.Ext(absSrc)
	stem := strings.TrimSuffix(filepath.Base(absSrc), ext)
	dst := filepath.Join(assets, stem+ext)
	for n := 2; fileExists(dst); n++ {
		dst = filepath.Join(assets, fmt.Sprintf("%s-%d%s", stem, n, ext))
	}
	if err := copyFile(absSrc, dst); err != nil {
		return "", err
	}
	return "assets/" + filepath.Base(dst), nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(dst)
		return fmt.Errorf("failed to copy %s: %w", filepath.Base(src), err)
	}
	return out.Close()
}

// FindFile locates a non-note file a link names ("diagram.png" or
// "assets/diagram.png"): next to fromID first, then from the workspace root,
// then anywhere in the workspace by name. It returns the absolute path.
func (s *Store) FindFile(name, fromID string) (string, bool) {
	name = strings.Trim(filepath.ToSlash(strings.TrimSpace(name)), "/")
	if name == "" || strings.Contains(name, "..") {
		return "", false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, rel := range []string{path.Join(path.Dir(fromID), name), name} {
		if abs := s.idToAbs(rel); fileExists(abs) {
			return abs, true
		}
	}
	base := strings.ToLower(path.Base(name))
	found := ""
	_ = filepath.WalkDir(s.root, func(p string, d os.DirEntry, err error) error {
		switch {
		case err != nil:
			return nil
		case strings.HasPrefix(d.Name(), ".") && p != s.root:
			if d.IsDir() {
				return filepath.SkipDir
			}
		case !d.IsDir() && strings.ToLower(d.Name()) == base:
			found = p
			return filepath.SkipAll
		}
		return nil
	})
	return found, found != ""
}

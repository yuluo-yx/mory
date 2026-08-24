package windowshost

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	maxDocumentBytes = 2 * 1024 * 1024
	maxImageBytes    = 50 * 1024 * 1024
)

var (
	documentExtensions = map[string]bool{".md": true, ".markdown": true, ".mmd": true, ".mdown": true, ".mkd": true, ".txt": true, ".text": true}
	imageExtensions    = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".svg": true, ".bmp": true}
	imageMIME          = map[string]string{"image/png": ".png", "image/jpeg": ".jpg", "image/gif": ".gif", "image/webp": ".webp", "image/svg+xml": ".svg", "image/bmp": ".bmp"}
	markdownImage      = regexp.MustCompile(`!\[[^\]]*\]\((?:<([^>]+)>|([^\s)]+))(?:\s+["'][^"']*["'])?\)`)
	rawHTMLImage       = regexp.MustCompile("(?is)<img\\b[^>]*\\bsrc\\s*=\\s*(?:\"([^\"]*)\"|'([^']*)'|([^\\s\"'=<>`]+))")
	unsafeSegment      = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1f\s]+`)
)

// Document is a note snapshot shared by the file tree and knowledge graph.
type Document struct {
	Name      string            `json:"name"`
	Path      string            `json:"path"`
	CreatedAt int64             `json:"createdAt"`
	UpdatedAt int64             `json:"updatedAt"`
	Size      int64             `json:"size"`
	Markdown  string            `json:"markdown,omitempty"`
	Images    []DocumentImage   `json:"images,omitempty"`
	Assets    map[string]string `json:"assets,omitempty"`
}

// DocumentImage describes an image stored in a note's matching asset directory.
type DocumentImage struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Relative  string `json:"relative"`
	UpdatedAt int64  `json:"updatedAt"`
	Size      int64  `json:"size"`
}

// Directory is a visible directory inside the active workspace.
type Directory struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	CreatedAt int64  `json:"createdAt"`
}

// WorkspaceMutation describes a workspace entry after creation, copy, or move.
type WorkspaceMutation struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	SourcePath  string `json:"sourcePath,omitempty"`
	IsDirectory bool   `json:"isDirectory"`
}

func listDocuments(root string, includeMarkdown bool) ([]Document, error) {
	documents := make([]Document, 0)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path != root && entry.IsDir() && hiddenWorkspaceEntry(entry.Name()) {
			return filepath.SkipDir
		}
		if entry.IsDir() || !documentExtensions[strings.ToLower(filepath.Ext(entry.Name()))] {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		document := Document{
			Name:      filepath.ToSlash(relative),
			Path:      path,
			CreatedAt: fileCreatedAt(info).UnixMilli(),
			UpdatedAt: info.ModTime().UnixMilli(),
			Size:      info.Size(),
		}
		document.Images, err = listDocumentImages(path)
		if err != nil {
			return err
		}
		if includeMarkdown && info.Size() <= maxDocumentBytes {
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				if errors.Is(readErr, os.ErrNotExist) {
					return nil
				}
				return readErr
			}
			document.Markdown = string(data)
		}
		documents = append(documents, document)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("扫描工作区文稿：%w", err)
	}
	sortDocuments(documents)
	return documents, nil
}

func sortDocuments(documents []Document) {
	sort.SliceStable(documents, func(i, j int) bool {
		if documents[i].CreatedAt != documents[j].CreatedAt {
			return documents[i].CreatedAt < documents[j].CreatedAt
		}
		return naturalLess(documents[i].Name, documents[j].Name)
	})
}

func listDirectories(root string) ([]Directory, error) {
	directories := make([]Directory, 0)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		if entry.IsDir() && hiddenWorkspaceEntry(entry.Name()) {
			return filepath.SkipDir
		}
		if !entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		directories = append(directories, Directory{Name: filepath.ToSlash(relative), Path: path, CreatedAt: fileCreatedAt(info).UnixMilli()})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("扫描工作区目录：%w", err)
	}
	sort.SliceStable(directories, func(i, j int) bool { return naturalLess(directories[i].Name, directories[j].Name) })
	return directories, nil
}

func listDocumentImages(documentPath string) ([]DocumentImage, error) {
	assetRoot := filepath.Join(filepath.Dir(documentPath), sanitizeSegment(strings.TrimSuffix(filepath.Base(documentPath), filepath.Ext(documentPath))))
	images := make([]DocumentImage, 0)
	err := filepath.WalkDir(assetRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if errors.Is(walkErr, os.ErrNotExist) {
			return nil
		}
		if walkErr != nil {
			return walkErr
		}
		if path != assetRoot && entry.IsDir() && strings.HasPrefix(entry.Name(), ".") {
			return filepath.SkipDir
		}
		if entry.IsDir() || !imageExtensions[strings.ToLower(filepath.Ext(entry.Name()))] {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		name, err := filepath.Rel(assetRoot, path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(filepath.Dir(documentPath), path)
		if err != nil {
			return err
		}
		images = append(images, DocumentImage{
			Name:      filepath.ToSlash(name),
			Path:      path,
			Relative:  filepath.ToSlash(relative),
			UpdatedAt: info.ModTime().UnixMilli(),
			Size:      info.Size(),
		})
		return nil
	})
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("扫描文稿图片：%w", err)
	}
	sort.SliceStable(images, func(i, j int) bool { return naturalLess(images[i].Name, images[j].Name) })
	return images, nil
}

func loadDocument(path string) (Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Document{}, fmt.Errorf("读取文稿：%w", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return Document{}, fmt.Errorf("读取文稿信息：%w", err)
	}
	document := Document{
		Name:      filepath.Base(path),
		Path:      path,
		CreatedAt: fileCreatedAt(info).UnixMilli(),
		UpdatedAt: info.ModTime().UnixMilli(),
		Size:      info.Size(),
		Markdown:  string(data),
	}
	document.Images, err = listDocumentImages(path)
	if err != nil {
		return Document{}, err
	}
	document.Assets = loadDocumentAssets(path, document.Markdown)
	return document, nil
}

func loadDocumentAssets(documentPath, markdown string) map[string]string {
	assets := make(map[string]string)
	for _, reference := range documentImageReferences(markdown) {
		resolved, err := safeDescendant(filepath.Dir(documentPath), filepath.FromSlash(reference))
		if err != nil {
			continue
		}
		data, err := os.ReadFile(resolved)
		if err != nil || len(data) > maxImageBytes {
			continue
		}
		assets[filepath.ToSlash(reference)] = dataURL(resolved, data)
	}
	return assets
}

func documentImageReferences(markdown string) []string {
	references := make([]string, 0)
	seen := make(map[string]bool)
	collect := func(matches [][]string) {
		for _, match := range matches {
			reference := ""
			for _, candidate := range match[1:] {
				if candidate != "" {
					reference = candidate
					break
				}
			}
			if decoded, err := url.PathUnescape(reference); err == nil {
				reference = decoded
			}
			if reference == "" || hasRemoteScheme(reference) || seen[reference] {
				continue
			}
			seen[reference] = true
			references = append(references, reference)
		}
	}
	collect(markdownImage.FindAllStringSubmatch(markdown, -1))
	collect(rawHTMLImage.FindAllStringSubmatch(markdown, -1))
	return references
}

func readDocumentImage(root, path string) (map[string]any, error) {
	resolved, err := safeExistingPath(root, path)
	if err != nil {
		return nil, err
	}
	if !imageExtensions[strings.ToLower(filepath.Ext(resolved))] {
		return nil, errors.New("不支持的图片格式")
	}
	data, err := os.ReadFile(resolved)
	if err != nil {
		return nil, fmt.Errorf("读取图片：%w", err)
	}
	if len(data) > maxImageBytes {
		return nil, errors.New("图片超过 50 MB")
	}
	return map[string]any{"name": filepath.Base(resolved), "path": resolved, "dataURL": dataURL(resolved, data)}, nil
}

func importImage(root, documentPath, documentName, name, mimeType, encoded string) (map[string]any, error) {
	extension, ok := imageMIME[mimeType]
	if !ok {
		return nil, errors.New("仅支持 PNG、JPEG、GIF、WebP、SVG 和 BMP 图片")
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(data) == 0 || len(data) > maxImageBytes {
		return nil, errors.New("图片为空、编码无效或超过 50 MB")
	}
	documentBase := sanitizeSegment(strings.TrimSuffix(filepath.Base(documentName), filepath.Ext(documentName)))
	documentDirectory := root
	if documentPath != "" {
		resolved, resolveErr := safeExistingPath(root, documentPath)
		if resolveErr != nil {
			return nil, resolveErr
		}
		documentDirectory = filepath.Dir(resolved)
	}
	assetDirectory := filepath.Join(documentDirectory, documentBase)
	if err := os.MkdirAll(assetDirectory, 0o755); err != nil {
		return nil, fmt.Errorf("创建图片目录：%w", err)
	}
	originalBase := sanitizeSegment(strings.TrimSuffix(filepath.Base(name), filepath.Ext(name)))
	for serial := 1; ; serial++ {
		filename := originalBase + extension
		if serial > 1 {
			filename = fmt.Sprintf("%s-%d%s", originalBase, serial, extension)
		}
		file, createErr := os.OpenFile(filepath.Join(assetDirectory, filename), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if errors.Is(createErr, fs.ErrExist) {
			continue
		}
		if createErr != nil {
			return nil, fmt.Errorf("创建图片：%w", createErr)
		}
		_, writeErr := file.Write(data)
		closeErr := file.Close()
		if writeErr != nil {
			return nil, fmt.Errorf("写入图片：%w", writeErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("关闭图片：%w", closeErr)
		}
		relative := filepath.ToSlash(filepath.Join(documentBase, filename))
		return map[string]any{"relative": relative, "dataURL": "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data)}, nil
	}
}

func relocateDocumentAssets(root, markdown, oldPath, oldName, newPath string) (string, error) {
	oldBase := sanitizeSegment(strings.TrimSuffix(filepath.Base(oldName), filepath.Ext(oldName)))
	newBase := sanitizeSegment(strings.TrimSuffix(filepath.Base(newPath), filepath.Ext(newPath)))
	oldParent := root
	if oldPath != "" {
		oldParent = filepath.Dir(oldPath)
	}
	if oldBase == newBase && filepath.Clean(oldParent) == filepath.Clean(filepath.Dir(newPath)) {
		return markdown, nil
	}
	oldDirectory := filepath.Join(oldParent, oldBase)
	newDirectory := filepath.Join(filepath.Dir(newPath), newBase)
	if _, err := os.Stat(oldDirectory); errors.Is(err, os.ErrNotExist) {
		return markdown, nil
	} else if err != nil {
		return "", fmt.Errorf("读取原图片目录：%w", err)
	}
	if err := os.MkdirAll(filepath.Dir(newDirectory), 0o755); err != nil {
		return "", fmt.Errorf("创建新图片目录：%w", err)
	}
	if err := os.Rename(oldDirectory, newDirectory); err != nil {
		if copyErr := copyDirectory(oldDirectory, newDirectory); copyErr != nil {
			return "", fmt.Errorf("迁移文稿图片：%w", errors.Join(err, copyErr))
		}
	}
	return strings.ReplaceAll(markdown, "]("+oldBase+"/", "]("+newBase+"/"), nil
}

func copyDirectory(source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}

func createWorkspaceDirectory(root, relative string) (Directory, error) {
	if strings.TrimSpace(relative) == "" || filepath.IsAbs(relative) {
		return Directory{}, errors.New("请输入工作区内的相对目录")
	}
	resolved, err := safeDescendant(root, filepath.Clean(filepath.FromSlash(relative)))
	if err != nil || resolved == root {
		return Directory{}, errors.New("目录路径不能包含空层级、. 或 ..")
	}
	if err := os.MkdirAll(resolved, 0o755); err != nil {
		return Directory{}, fmt.Errorf("创建目录：%w", err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return Directory{}, fmt.Errorf("读取目录信息：%w", err)
	}
	name, _ := filepath.Rel(root, resolved)
	return Directory{Name: filepath.ToSlash(name), Path: resolved, CreatedAt: fileCreatedAt(info).UnixMilli()}, nil
}

func createWorkspaceDocument(root, directory, name string) (Document, error) {
	destination, err := workspaceDirectory(root, directory)
	if err != nil {
		return Document{}, err
	}
	name = sanitizeSegment(strings.TrimSuffix(filepath.Base(name), filepath.Ext(name))) + ".md"
	path := availableEntryPath(destination, name, false)
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		return Document{}, fmt.Errorf("创建文稿：%w", err)
	}
	return loadDocument(path)
}

func copyWorkspaceEntry(root, source, destination string) (WorkspaceMutation, error) {
	resolved, info, targetDirectory, err := workspaceEntryPaths(root, source, destination)
	if err != nil {
		return WorkspaceMutation{}, err
	}
	if info.IsDir() && isSameOrDescendant(resolved, targetDirectory) {
		return WorkspaceMutation{}, errors.New("不能把目录复制到自身或子目录")
	}
	target := availableEntryPath(targetDirectory, filepath.Base(resolved), info.IsDir())
	if info.IsDir() {
		err = copyDirectory(resolved, target)
	} else {
		err = copyFile(resolved, target)
		if err == nil {
			err = copyCompanionAssets(resolved, target)
		}
	}
	if err != nil {
		return WorkspaceMutation{}, fmt.Errorf("复制工作区条目：%w", err)
	}
	name, _ := filepath.Rel(root, target)
	return WorkspaceMutation{Name: filepath.ToSlash(name), Path: target, SourcePath: resolved, IsDirectory: info.IsDir()}, nil
}

func moveWorkspaceEntry(root, source, destination string) (WorkspaceMutation, error) {
	resolved, info, targetDirectory, err := workspaceEntryPaths(root, source, destination)
	if err != nil {
		return WorkspaceMutation{}, err
	}
	if filepath.Clean(filepath.Dir(resolved)) == filepath.Clean(targetDirectory) {
		return WorkspaceMutation{}, errors.New("条目已经位于所选目录")
	}
	if info.IsDir() && isSameOrDescendant(resolved, targetDirectory) {
		return WorkspaceMutation{}, errors.New("不能把目录移动到自身或子目录")
	}
	target := availableEntryPath(targetDirectory, filepath.Base(resolved), info.IsDir())
	if err := movePath(resolved, target, info.IsDir()); err != nil {
		return WorkspaceMutation{}, fmt.Errorf("移动工作区条目：%w", err)
	}
	if !info.IsDir() {
		if err := moveCompanionAssets(resolved, target); err != nil {
			return WorkspaceMutation{}, err
		}
	}
	name, _ := filepath.Rel(root, target)
	return WorkspaceMutation{Name: filepath.ToSlash(name), Path: target, SourcePath: resolved, IsDirectory: info.IsDir()}, nil
}

func renameWorkspaceEntry(root, source, requestedName string) (WorkspaceMutation, error) {
	resolved, err := safeExistingPath(root, source)
	if err != nil {
		return WorkspaceMutation{}, err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return WorkspaceMutation{}, fmt.Errorf("读取工作区条目：%w", err)
	}
	requestedName = strings.TrimSpace(requestedName)
	if requestedName == "" || strings.ContainsAny(requestedName, `/\`) || filepath.Base(requestedName) != requestedName || requestedName == "." || requestedName == ".." {
		return WorkspaceMutation{}, errors.New("名称不能为空或包含路径分隔符")
	}
	extension := ""
	base := requestedName
	if !info.IsDir() {
		extension = filepath.Ext(requestedName)
		base = strings.TrimSuffix(requestedName, extension)
		if extension == "" {
			extension = filepath.Ext(resolved)
		}
	}
	filename := sanitizeSegment(base) + extension
	target := filepath.Join(filepath.Dir(resolved), filename)
	if filepath.Clean(target) == filepath.Clean(resolved) {
		return WorkspaceMutation{}, errors.New("名称没有变化")
	}
	if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		if err == nil {
			return WorkspaceMutation{}, errors.New("同名条目已经存在")
		}
		return WorkspaceMutation{}, fmt.Errorf("检查重命名目标：%w", err)
	}
	if info.IsDir() {
		if err := os.Rename(resolved, target); err != nil {
			return WorkspaceMutation{}, fmt.Errorf("重命名目录：%w", err)
		}
		name, _ := filepath.Rel(root, target)
		return WorkspaceMutation{Name: filepath.ToSlash(name), Path: target, SourcePath: resolved, IsDirectory: true}, nil
	}

	sourceAssets := companionAssets(resolved)
	targetAssets := companionAssets(target)
	_, assetsErr := os.Stat(sourceAssets)
	hasAssets := assetsErr == nil
	if assetsErr != nil && !errors.Is(assetsErr, os.ErrNotExist) {
		return WorkspaceMutation{}, fmt.Errorf("检查文稿图片目录：%w", assetsErr)
	}
	if hasAssets {
		if _, err := os.Stat(targetAssets); !errors.Is(err, os.ErrNotExist) {
			if err == nil {
				return WorkspaceMutation{}, errors.New("同名图片目录已经存在")
			}
			return WorkspaceMutation{}, fmt.Errorf("检查目标图片目录：%w", err)
		}
	}
	markdown, err := os.ReadFile(resolved)
	if err != nil {
		return WorkspaceMutation{}, fmt.Errorf("读取待重命名文稿：%w", err)
	}
	nextMarkdown := strings.ReplaceAll(string(markdown), "]("+filepath.Base(sourceAssets)+"/", "]("+filepath.Base(targetAssets)+"/")
	if err := os.Rename(resolved, target); err != nil {
		return WorkspaceMutation{}, fmt.Errorf("重命名文稿：%w", err)
	}
	assetsMoved := false
	if hasAssets {
		if err := os.Rename(sourceAssets, targetAssets); err != nil {
			_ = os.Rename(target, resolved)
			return WorkspaceMutation{}, fmt.Errorf("重命名文稿图片目录：%w", err)
		}
		assetsMoved = true
	}
	if nextMarkdown != string(markdown) {
		if err := os.WriteFile(target, []byte(nextMarkdown), info.Mode().Perm()); err != nil {
			if assetsMoved {
				_ = os.Rename(targetAssets, sourceAssets)
			}
			_ = os.Rename(target, resolved)
			return WorkspaceMutation{}, fmt.Errorf("更新文稿图片引用：%w", err)
		}
	}
	name, _ := filepath.Rel(root, target)
	return WorkspaceMutation{Name: filepath.ToSlash(name), Path: target, SourcePath: resolved, IsDirectory: false}, nil
}

func workspaceEntryPaths(root, source, destination string) (string, fs.FileInfo, string, error) {
	resolved, err := safeExistingPath(root, source)
	if err != nil {
		return "", nil, "", err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", nil, "", fmt.Errorf("读取工作区条目：%w", err)
	}
	targetDirectory, err := workspaceDirectory(root, destination)
	return resolved, info, targetDirectory, err
}

func workspaceDirectory(root, value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return filepath.Abs(root)
	}
	resolved, err := safeExistingPath(root, value)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.IsDir() {
		return "", errors.New("目标必须是当前工作区中的目录")
	}
	return resolved, nil
}

func availableEntryPath(directory, name string, isDirectory bool) string {
	extension := ""
	base := name
	if !isDirectory {
		extension = filepath.Ext(name)
		base = strings.TrimSuffix(name, extension)
	}
	for serial := 1; ; serial++ {
		candidateName := name
		if serial > 1 {
			suffix := " 副本"
			if serial > 2 {
				suffix = fmt.Sprintf(" 副本 %d", serial-1)
			}
			candidateName = base + suffix + extension
		}
		candidate := filepath.Join(directory, candidateName)
		_, entryErr := os.Stat(candidate)
		_, assetsErr := os.Stat(companionAssets(candidate))
		if errors.Is(entryErr, os.ErrNotExist) && (isDirectory || errors.Is(assetsErr, os.ErrNotExist)) {
			return candidate
		}
	}
}

func copyFile(source, destination string) error {
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	return os.WriteFile(destination, data, 0o644)
}

func movePath(source, destination string, isDirectory bool) error {
	if err := os.Rename(source, destination); err == nil {
		return nil
	}
	if isDirectory {
		if err := copyDirectory(source, destination); err != nil {
			return err
		}
		return os.RemoveAll(source)
	}
	if err := copyFile(source, destination); err != nil {
		return err
	}
	return os.Remove(source)
}

func companionAssets(path string) string {
	return filepath.Join(filepath.Dir(path), sanitizeSegment(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))))
}

func copyCompanionAssets(source, destination string) error {
	assets := companionAssets(source)
	if info, err := os.Stat(assets); errors.Is(err, os.ErrNotExist) || err == nil && !info.IsDir() {
		return nil
	} else if err != nil {
		return err
	}
	return copyDirectory(assets, companionAssets(destination))
}

func moveCompanionAssets(source, destination string) error {
	assets := companionAssets(source)
	info, err := os.Stat(assets)
	if errors.Is(err, os.ErrNotExist) || err == nil && !info.IsDir() {
		return nil
	}
	if err != nil {
		return err
	}
	if err := movePath(assets, companionAssets(destination), true); err != nil {
		return fmt.Errorf("移动文稿图片：%w", err)
	}
	return nil
}

func isSameOrDescendant(parent, value string) bool {
	relative, err := filepath.Rel(parent, value)
	return err == nil && (relative == "." || relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)))
}

func safeExistingPath(root, value string) (string, error) {
	resolved, err := filepath.Abs(value)
	if err != nil {
		return "", fmt.Errorf("解析路径：%w", err)
	}
	if _, err := safeDescendant(root, resolved); err != nil {
		return "", err
	}
	return resolved, nil
}

func safeDescendant(root, value string) (string, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("解析工作区：%w", err)
	}
	resolved := value
	if !filepath.IsAbs(value) {
		resolved = filepath.Join(root, value)
	}
	resolved, err = filepath.Abs(resolved)
	if err != nil {
		return "", fmt.Errorf("解析文件：%w", err)
	}
	relative, err := filepath.Rel(root, resolved)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", errors.New("文件必须位于当前工作区内")
	}
	return resolved, nil
}

func sanitizeSegment(value string) string {
	value = unsafeSegment.ReplaceAllString(value, "-")
	value = strings.Trim(strings.TrimSpace(value), ".")
	if value == "" {
		return "未命名"
	}
	return value
}

func hiddenWorkspaceEntry(name string) bool {
	return name == ".git" || name == ".mory" || strings.HasPrefix(name, ".")
}

func hasRemoteScheme(reference string) bool {
	lower := strings.ToLower(reference)
	return strings.HasPrefix(lower, "data:") || strings.HasPrefix(lower, "http:") || strings.HasPrefix(lower, "https:") || strings.HasPrefix(lower, "file:") || strings.HasPrefix(reference, "/")
}

func dataURL(path string, data []byte) string {
	return "data:" + mimeTypeForPath(path) + ";base64," + base64.StdEncoding.EncodeToString(data)
}

func naturalLess(left, right string) bool {
	left, right = strings.ToLower(left), strings.ToLower(right)
	for len(left) > 0 && len(right) > 0 {
		leftDigits := left[0] >= '0' && left[0] <= '9'
		rightDigits := right[0] >= '0' && right[0] <= '9'
		if leftDigits && rightDigits {
			leftRun, leftRest := digitRun(left)
			rightRun, rightRest := digitRun(right)
			leftNumber, _ := strconv.ParseUint(leftRun, 10, 64)
			rightNumber, _ := strconv.ParseUint(rightRun, 10, 64)
			if leftNumber != rightNumber {
				return leftNumber < rightNumber
			}
			left, right = leftRest, rightRest
			continue
		}
		if left[0] != right[0] {
			return left[0] < right[0]
		}
		left, right = left[1:], right[1:]
	}
	return len(left) < len(right)
}

func digitRun(value string) (string, string) {
	index := 0
	for index < len(value) && value[index] >= '0' && value[index] <= '9' {
		index++
	}
	return value[:index], value[index:]
}

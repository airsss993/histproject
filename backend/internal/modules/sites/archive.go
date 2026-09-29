package sites

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
)

var (
	ErrArchiveTooLarge   = errors.New("archive is too large")
	ErrDuplicatePath     = errors.New("archive contains duplicate path")
	ErrExtractedTooLarge = errors.New("archive expands beyond the limit")
	ErrFileTooLarge      = errors.New("archive file is too large")
	ErrIndexMissing      = errors.New("root index.html is missing")
	ErrInvalidArchive    = errors.New("invalid ZIP archive")
	ErrInvalidLimits     = errors.New("invalid archive limits")
	ErrSymlink           = errors.New("archive contains a symlink")
	ErrTooManyFiles      = errors.New("archive contains too many files")
	ErrUnsafePath        = errors.New("archive contains an unsafe path")
	ErrUnsupportedFile   = errors.New("archive contains an unsupported file type")
)

// Сайт статический и отдаётся из MinIO под CSP sandbox, серверный код в нём не исполняется.
// Исполняемые файлы режем, чтобы архив не служил раздачей вредоносов с нашего домена.
// ponytail: список по расширению, антивирус (ClamAV) подключать, если понадобится проверка содержимого.
var executableExtensions = map[string]bool{
	".exe": true, ".dll": true, ".com": true, ".scr": true, ".msi": true, ".bat": true, ".cmd": true,
	".ps1": true, ".vbs": true, ".jar": true, ".apk": true, ".sh": true, ".php": true, ".phtml": true,
	".py": true, ".pl": true, ".cgi": true, ".asp": true, ".aspx": true, ".jsp": true, ".htaccess": true,
}

type Limits struct {
	MaxArchiveBytes   int64
	MaxExtractedBytes int64
	MaxFiles          int
	MaxFileBytes      int64
}

type Archive struct {
	files []File
}

type File struct {
	Path string
	Size int64
	file *zip.File
}

func OpenArchive(ctx context.Context, reader io.ReaderAt, size int64, limits Limits) (*Archive, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if limits.MaxArchiveBytes <= 0 || limits.MaxExtractedBytes <= 0 || limits.MaxFiles <= 0 || limits.MaxFileBytes <= 0 {
		return nil, ErrInvalidLimits
	}
	if size > limits.MaxArchiveBytes {
		return nil, ErrArchiveTooLarge
	}
	if size < 0 {
		return nil, ErrInvalidArchive
	}

	zipReader, err := zip.NewReader(reader, size)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidArchive, err)
	}
	if len(zipReader.File) > limits.MaxFiles {
		return nil, ErrTooManyFiles
	}

	archive := &Archive{files: make([]File, 0, len(zipReader.File))}
	paths := make(map[string]struct{}, len(zipReader.File))
	var extractedBytes uint64
	hasIndex := false

	for _, zipFile := range zipReader.File {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		name := zipFile.Name
		if !utf8.ValidString(name) {
			name, err = charmap.CodePage866.NewDecoder().String(name)
			if err != nil {
				return nil, fmt.Errorf("%w: decode file name: %v", ErrInvalidArchive, err)
			}
		}
		cleanPath, err := safeArchivePath(name)
		if err != nil {
			return nil, err
		}
		if cleanPath == "." && zipFile.FileInfo().IsDir() {
			continue
		}
		if _, exists := paths[cleanPath]; exists {
			return nil, fmt.Errorf("%w: %s", ErrDuplicatePath, cleanPath)
		}
		paths[cleanPath] = struct{}{}

		mode := zipFile.Mode()
		if mode&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("%w: %s", ErrSymlink, cleanPath)
		}
		if zipFile.FileInfo().IsDir() {
			continue
		}
		if !mode.IsRegular() || executableExtensions[strings.ToLower(path.Ext(cleanPath))] {
			return nil, fmt.Errorf("%w: %s", ErrUnsupportedFile, cleanPath)
		}
		if zipFile.UncompressedSize64 > uint64(limits.MaxFileBytes) {
			return nil, fmt.Errorf("%w: %s", ErrFileTooLarge, cleanPath)
		}
		if zipFile.UncompressedSize64 > uint64(limits.MaxExtractedBytes)-extractedBytes {
			return nil, ErrExtractedTooLarge
		}

		extractedBytes += zipFile.UncompressedSize64
		archive.files = append(archive.files, File{
			Path: cleanPath,
			Size: int64(zipFile.UncompressedSize64),
			file: zipFile,
		})
		if cleanPath == "index.html" {
			hasIndex = true
		}
	}

	if !hasIndex {
		return nil, ErrIndexMissing
	}
	return archive, nil
}

func (archive *Archive) Files() []File {
	return append([]File(nil), archive.files...)
}

func (file File) Open() (io.ReadCloser, error) {
	return file.file.Open()
}

func safeArchivePath(name string) (string, error) {
	if name == "" || strings.ContainsRune(name, 0) || strings.Contains(name, "\\") || path.IsAbs(name) || hasWindowsDrivePrefix(name) {
		return "", fmt.Errorf("%w: %q", ErrUnsafePath, name)
	}

	cleanPath := path.Clean(strings.TrimSuffix(name, "/"))
	if cleanPath == ".." || strings.HasPrefix(cleanPath, "../") {
		return "", fmt.Errorf("%w: %q", ErrUnsafePath, name)
	}
	return cleanPath, nil
}

func hasWindowsDrivePrefix(name string) bool {
	return len(name) >= 2 && unicode.IsLetter(rune(name[0])) && name[1] == ':'
}

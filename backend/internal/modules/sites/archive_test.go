package sites

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"os"
	"testing"
)

func TestOpenArchiveAcceptsSafeSite(t *testing.T) {
	data := makeZIP(t,
		zipEntry{name: "index.html", body: "<h1>site</h1>"},
		zipEntry{name: "assets/", mode: os.ModeDir | 0o755},
		zipEntry{name: "assets/app.js", body: "console.log('ok')"},
	)

	archive, err := OpenArchive(t.Context(), bytes.NewReader(data), int64(len(data)), testLimits())
	if err != nil {
		t.Fatalf("OpenArchive() error = %v", err)
	}
	files := archive.Files()
	if len(files) != 2 {
		t.Fatalf("len(Files()) = %d, want 2", len(files))
	}
	if files[0].Path != "index.html" || files[1].Path != "assets/app.js" {
		t.Fatalf("Files() paths = %q, %q", files[0].Path, files[1].Path)
	}
	if files[0].Size != int64(len("<h1>site</h1>")) {
		t.Fatalf("index size = %d", files[0].Size)
	}
	reader, err := files[1].Open()
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	var content bytes.Buffer
	_, readErr := content.ReadFrom(reader)
	closeErr := reader.Close()
	if readErr != nil {
		t.Fatalf("read file: %v", readErr)
	}
	if closeErr != nil {
		t.Fatalf("close file: %v", closeErr)
	}
	if content.String() != "console.log('ok')" {
		t.Fatalf("file content = %q", content.String())
	}
}

func TestOpenArchiveRequiresRootIndex(t *testing.T) {
	data := makeZIP(t, zipEntry{name: "site/index.html", body: "site"})

	_, err := OpenArchive(t.Context(), bytes.NewReader(data), int64(len(data)), testLimits())
	if !errors.Is(err, ErrIndexMissing) {
		t.Fatalf("OpenArchive() error = %v, want ErrIndexMissing", err)
	}
}

func TestOpenArchiveRejectsUnsafePaths(t *testing.T) {
	paths := []string{
		"../evil.txt",
		"assets/../../evil.txt",
		"/absolute.txt",
		"C:/absolute.txt",
		"assets\\windows.txt",
	}
	for _, unsafePath := range paths {
		t.Run(unsafePath, func(t *testing.T) {
			data := makeZIP(t,
				zipEntry{name: "index.html", body: "site"},
				zipEntry{name: unsafePath, body: "evil"},
			)

			_, err := OpenArchive(t.Context(), bytes.NewReader(data), int64(len(data)), testLimits())
			if !errors.Is(err, ErrUnsafePath) {
				t.Fatalf("OpenArchive() error = %v, want ErrUnsafePath", err)
			}
		})
	}
}

func TestOpenArchiveRejectsSymlink(t *testing.T) {
	data := makeZIP(t,
		zipEntry{name: "index.html", body: "site"},
		zipEntry{name: "assets/link", body: "../../secret", mode: os.ModeSymlink | 0o777},
	)

	_, err := OpenArchive(t.Context(), bytes.NewReader(data), int64(len(data)), testLimits())
	if !errors.Is(err, ErrSymlink) {
		t.Fatalf("OpenArchive() error = %v, want ErrSymlink", err)
	}
}

func TestOpenArchiveRejectsDuplicateNormalizedPath(t *testing.T) {
	data := makeZIP(t,
		zipEntry{name: "index.html", body: "site"},
		zipEntry{name: "assets/app.js", body: "one"},
		zipEntry{name: "assets/./app.js", body: "two"},
	)

	_, err := OpenArchive(t.Context(), bytes.NewReader(data), int64(len(data)), testLimits())
	if !errors.Is(err, ErrDuplicatePath) {
		t.Fatalf("OpenArchive() error = %v, want ErrDuplicatePath", err)
	}
}

func TestOpenArchiveEnforcesLimits(t *testing.T) {
	tests := []struct {
		name   string
		limits Limits
		data   func(*testing.T) []byte
		want   error
	}{
		{
			name: "archive bytes",
			limits: Limits{
				MaxArchiveBytes: 1, MaxExtractedBytes: 100, MaxFiles: 10, MaxFileBytes: 100,
			},
			data: func(t *testing.T) []byte {
				return makeZIP(t, zipEntry{name: "index.html", body: "site"})
			},
			want: ErrArchiveTooLarge,
		},
		{
			name: "file count",
			limits: Limits{
				MaxArchiveBytes: 10_000, MaxExtractedBytes: 100, MaxFiles: 1, MaxFileBytes: 100,
			},
			data: func(t *testing.T) []byte {
				return makeZIP(t,
					zipEntry{name: "index.html", body: "site"},
					zipEntry{name: "app.js", body: "code"},
				)
			},
			want: ErrTooManyFiles,
		},
		{
			name: "single file bytes",
			limits: Limits{
				MaxArchiveBytes: 10_000, MaxExtractedBytes: 100, MaxFiles: 10, MaxFileBytes: 3,
			},
			data: func(t *testing.T) []byte {
				return makeZIP(t, zipEntry{name: "index.html", body: "site"})
			},
			want: ErrFileTooLarge,
		},
		{
			name: "total extracted bytes",
			limits: Limits{
				MaxArchiveBytes: 10_000, MaxExtractedBytes: 7, MaxFiles: 10, MaxFileBytes: 100,
			},
			data: func(t *testing.T) []byte {
				return makeZIP(t,
					zipEntry{name: "index.html", body: "site"},
					zipEntry{name: "app.js", body: "code"},
				)
			},
			want: ErrExtractedTooLarge,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data := test.data(t)
			_, err := OpenArchive(t.Context(), bytes.NewReader(data), int64(len(data)), test.limits)
			if !errors.Is(err, test.want) {
				t.Fatalf("OpenArchive() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestOpenArchiveRejectsCorruptZIP(t *testing.T) {
	data := []byte("not a zip")

	_, err := OpenArchive(t.Context(), bytes.NewReader(data), int64(len(data)), testLimits())
	if !errors.Is(err, ErrInvalidArchive) {
		t.Fatalf("OpenArchive() error = %v, want ErrInvalidArchive", err)
	}
}

func TestOpenArchiveStopsForCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	data := makeZIP(t, zipEntry{name: "index.html", body: "site"})

	_, err := OpenArchive(ctx, bytes.NewReader(data), int64(len(data)), testLimits())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("OpenArchive() error = %v, want context.Canceled", err)
	}
}

type zipEntry struct {
	name  string
	body  string
	mode  os.FileMode
	store bool
}

func makeZIP(t *testing.T, entries ...zipEntry) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, entry := range entries {
		header := &zip.FileHeader{Name: entry.name, Method: zip.Deflate}
		if entry.store {
			header.Method = zip.Store
		}
		if entry.mode != 0 {
			header.SetMode(entry.mode)
		}
		file, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatalf("create ZIP entry: %v", err)
		}
		if _, err := file.Write([]byte(entry.body)); err != nil {
			t.Fatalf("write ZIP entry: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close ZIP: %v", err)
	}
	return buffer.Bytes()
}

func testLimits() Limits {
	return Limits{
		MaxArchiveBytes:   10_000,
		MaxExtractedBytes: 1_000,
		MaxFiles:          10,
		MaxFileBytes:      500,
	}
}

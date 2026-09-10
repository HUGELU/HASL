package main

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type DownloadSpec struct {
	Role    string `json:"role,omitempty"`
	Name    string `json:"name"`
	URL     string `json:"url"`
	SHA256  string `json:"sha256"`
	Size    int64  `json:"size"`
	License string `json:"license"`
	Source  string `json:"source"`
}
type ModelCatalog struct {
	ID       string                  `json:"id"`
	Name     string                  `json:"name"`
	License  string                  `json:"license"`
	Files    []DownloadSpec          `json:"files"`
	Runtimes map[string]DownloadSpec `json:"runtimes"`
}

// A pinned full digest is the trust anchor. Partial files are never executable.
func checkDownload(ctx context.Context, path string, spec DownloadSpec) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return err
	}
	if st.Size() != spec.Size || !st.Mode().IsRegular() {
		return errors.New("file size differs from the pinned manifest")
	}
	h := sha256.New()
	buf := make([]byte, 1<<20)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, e := f.Read(buf)
		if n > 0 {
			h.Write(buf[:n])
		}
		if e == io.EOF {
			break
		}
		if e != nil {
			return e
		}
	}
	if hex.EncodeToString(h.Sum(nil)) != spec.SHA256 {
		return errors.New("SHA-256 verification failed")
	}
	return nil
}

func downloadPinned(ctx context.Context, client *http.Client, spec DownloadSpec, dest string, progress func(string, int64, int64)) error {
	if len(spec.SHA256) != 64 || spec.Size <= 0 {
		return errors.New("download manifest is missing a full checksum or length")
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
		return err
	}
	if _, err := os.Stat(dest); err == nil {
		progress("Verifying "+spec.Name, 0, spec.Size)
		if err := checkDownload(ctx, dest, spec); err == nil {
			progress("Verified "+spec.Name, spec.Size, spec.Size)
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("existing %s is damaged; remove this cache file and retry setup", spec.Name)
	}
	part := dest + ".part"
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		start := int64(0)
		if st, err := os.Stat(part); err == nil {
			start = st.Size()
		}
		if start > spec.Size {
			return errors.New("partial download is larger than the expected file; remove the .part file and retry")
		}
		if start == spec.Size {
			break
		}
		req, err := http.NewRequestWithContext(ctx, "GET", spec.URL, nil)
		if err != nil {
			return err
		}
		req.Header.Set("User-Agent", "ORIGIN0/1.6.1 (+https://github.com/HUGELU/HASL)")
		req.Header.Set("Accept-Encoding", "identity")
		if start > 0 {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-", start))
		}
		res, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		if res.StatusCode != 200 && res.StatusCode != 206 {
			res.Body.Close()
			return fmt.Errorf("download returned HTTP %d for %s; check the connection and retry", res.StatusCode, spec.Name)
		}
		if res.StatusCode == 206 {
			prefix := fmt.Sprintf("bytes %d-", start)
			value := res.Header.Get("Content-Range")
			if !strings.HasPrefix(value, prefix) || !strings.HasSuffix(value, "/"+strconv.FormatInt(spec.Size, 10)) {
				res.Body.Close()
				return errors.New("server returned an inconsistent resume range")
			}
		} else {
			start = 0
		}
		flags := os.O_CREATE | os.O_WRONLY
		if start > 0 {
			flags |= os.O_APPEND
		} else {
			flags |= os.O_TRUNC
		}
		f, err := os.OpenFile(part, flags, 0600)
		if err != nil {
			res.Body.Close()
			return err
		}
		written := start
		buf := make([]byte, 1<<20)
		last := time.Time{}
		progress("Downloading "+spec.Name, written, spec.Size)
		for {
			n, readErr := res.Body.Read(buf)
			if n > 0 {
				if written+int64(n) > spec.Size {
					lastErr = errors.New("download exceeds its pinned length")
					break
				}
				count, writeErr := f.Write(buf[:n])
				written += int64(count)
				if writeErr != nil {
					lastErr = writeErr
					break
				}
				if time.Since(last) > 200*time.Millisecond {
					progress("Downloading "+spec.Name, written, spec.Size)
					last = time.Now()
				}
			}
			if readErr != nil {
				if readErr != io.EOF {
					lastErr = readErr
				} else if written != spec.Size {
					lastErr = io.ErrUnexpectedEOF
				} else {
					lastErr = nil
				}
				break
			}
		}
		syncErr := f.Sync()
		closeErr := f.Close()
		res.Body.Close()
		if syncErr != nil {
			return syncErr
		}
		if closeErr != nil {
			return closeErr
		}
		if lastErr == nil {
			break
		}
	}
	if lastErr != nil {
		return fmt.Errorf("download interrupted (retry resumes it): %w", lastErr)
	}
	progress("Verifying "+spec.Name, 0, spec.Size)
	if err := checkDownload(ctx, part, spec); err != nil {
		if ctx.Err() == nil {
			_ = os.Remove(part)
		}
		return err
	}
	if err := os.Rename(part, dest); err != nil {
		return err
	}
	progress("Verified "+spec.Name, spec.Size, spec.Size)
	return nil
}

// Extract only regular files from the checksum-verified native runtime archive.
func extractRuntime(path, dest string) error {
	z, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer z.Close()
	if len(z.File) > 2048 {
		return errors.New("runtime archive contains too many entries")
	}
	var total uint64
	for _, f := range z.File {
		name := strings.ReplaceAll(f.Name, "\\", "/")
		if name == "" || strings.HasPrefix(name, "/") || strings.Contains(name, ":") {
			return errors.New("invalid runtime archive path")
		}
		clean := filepath.Clean(filepath.FromSlash(name))
		if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return errors.New("runtime archive path escapes its folder")
		}
		if f.Mode()&os.ModeSymlink != 0 {
			return errors.New("runtime archive symlinks are not supported")
		}
		out := filepath.Join(dest, clean)
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(out, 0700); err != nil {
				return err
			}
			continue
		}
		total += f.UncompressedSize64
		if total > 2<<30 || f.UncompressedSize64 > 1<<30 {
			return errors.New("runtime extraction size limit exceeded")
		}
		if err := os.MkdirAll(filepath.Dir(out), 0700); err != nil {
			return err
		}
		r, err := f.Open()
		if err != nil {
			return err
		}
		w, err := os.OpenFile(out, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0700)
		if err != nil {
			r.Close()
			return err
		}
		n, copyErr := io.Copy(w, io.LimitReader(r, int64(f.UncompressedSize64)+1))
		wErr := w.Close()
		r.Close()
		if copyErr != nil {
			return copyErr
		}
		if wErr != nil {
			return wErr
		}
		if uint64(n) != f.UncompressedSize64 {
			return errors.New("runtime archive entry size mismatch")
		}
	}
	return nil
}

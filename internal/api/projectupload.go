package api

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/nikiv/ansible-ui/internal/model"
	zip "github.com/yeka/zip" // archive/zip fork with ZipCrypto + AES password support
)

const (
	maxUploadMemory = 32 << 20  // multipart in-memory threshold; larger spools to a temp file
	maxArchiveBytes = 1 << 30   // 1 GiB hard cap on the uploaded archive
	maxFileBytes    = 512 << 20 // per-entry cap during extraction (zip-bomb guard)
)

// errNeedPassword: the zip is encrypted and no password was supplied.
// errWrongPassword: a password was supplied but it didn't decrypt the archive.
var (
	errNeedPassword  = errors.New("the archive is password-protected — enter its password")
	errWrongPassword = errors.New("incorrect password for this archive")
)

// handleUploadArchive extracts an uploaded .tar.gz / .tgz / .tar / .zip into a
// local project's directory — a Git-free way to seed a project from a folder.
// Format is detected by magic bytes. Encrypted (password-protected) zips are
// supported: the UI re-submits with a `password` field.
func (s *Server) handleUploadArchive(w http.ResponseWriter, r *http.Request) {
	p, err := s.store.GetProject(r.Context(), r.PathValue("id"))
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	if p.RepositoryID != nil && *p.RepositoryID != "" {
		writeErr(w, http.StatusBadRequest, "project is git-backed — files come from the repository")
		return
	}
	if !s.requireProjectCap(w, r, p.ID, model.CapEdit) {
		return
	}
	if err := r.ParseMultipartForm(maxUploadMemory); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid upload: "+err.Error())
		return
	}
	password := r.FormValue("password")
	f, hdr, err := r.FormFile("file")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "no file in upload")
		return
	}
	defer f.Close()
	if hdr.Size > maxArchiveBytes {
		writeErr(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("archive too large (%d MiB, max %d MiB)", hdr.Size>>20, maxArchiveBytes>>20))
		return
	}

	// Sniff the first bytes for the format. The ZIP path uses ReaderAt (absolute
	// offsets) so the sequential read here is harmless; the tar path re-prepends.
	magic := make([]byte, 4)
	n, _ := io.ReadFull(f, magic)
	magic = magic[:n]
	isZip := n >= 4 && magic[0] == 'P' && magic[1] == 'K' && magic[2] == 0x03 && magic[3] == 0x04
	isGz := n >= 2 && magic[0] == 0x1f && magic[1] == 0x8b

	dest := s.projectDir(p)
	if err := os.MkdirAll(dest, 0o755); err != nil {
		writeErr(w, http.StatusInternalServerError, "prepare dir: "+err.Error())
		return
	}

	var count int
	switch {
	case isZip:
		count, err = extractZip(f, hdr.Size, dest, password)
	case isGz:
		var gz *gzip.Reader
		if gz, err = gzip.NewReader(io.MultiReader(bytes.NewReader(magic), f)); err == nil {
			count, err = extractTar(tar.NewReader(gz), dest)
			gz.Close()
		}
	default: // assume a plain (uncompressed) tar
		count, err = extractTar(tar.NewReader(io.MultiReader(bytes.NewReader(magic), f)), dest)
	}

	// Encrypted-zip handshake: tell the UI to ask for (or re-ask for) the password.
	if errors.Is(err, errNeedPassword) || errors.Is(err, errWrongPassword) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error(), "encrypted": true})
		return
	}
	if err != nil {
		writeErr(w, http.StatusBadRequest, "extract failed (expected .tar.gz / .tar / .zip): "+err.Error())
		return
	}
	if count == 0 {
		writeErr(w, http.StatusBadRequest, "no files found in the archive")
		return
	}
	s.recordActivity(r.Context(), s.actor(r.Context()), "project.upload", p.Name, hdr.Filename)
	writeJSON(w, http.StatusOK, map[string]any{"extracted": count})
}

func extractTar(tr *tar.Reader, dest string) (int, error) {
	n := 0
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return n, err
		}
		target, err := safeJoin(dest, h.Name)
		if err != nil {
			return n, err
		}
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return n, err
			}
		case tar.TypeReg:
			if err := writeArchiveFile(target, tr); err != nil {
				return n, err
			}
			n++
		}
	}
	return n, nil
}

func extractZip(ra io.ReaderAt, size int64, dest, password string) (int, error) {
	zr, err := zip.NewReader(ra, size)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, zf := range zr.File {
		encrypted := zf.IsEncrypted()
		if encrypted {
			if password == "" {
				return n, errNeedPassword
			}
			zf.SetPassword(password)
		}
		target, err := safeJoin(dest, zf.Name)
		if err != nil {
			return n, err
		}
		if zf.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return n, err
			}
			continue
		}
		rc, err := zf.Open()
		if err != nil {
			if encrypted {
				return n, errWrongPassword
			}
			return n, err
		}
		err = writeArchiveFile(target, rc)
		rc.Close()
		if err != nil {
			if encrypted { // a decrypt/CRC failure here means the password was wrong
				return n, errWrongPassword
			}
			return n, err
		}
		n++
	}
	return n, nil
}

func writeArchiveFile(target string, src io.Reader) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	out, err := os.Create(target)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, io.LimitReader(src, maxFileBytes))
	return err
}

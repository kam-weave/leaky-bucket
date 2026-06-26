package api

import (
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/weave-lab/interview-public/go/internal/auth"
	"github.com/weave-lab/interview-public/go/internal/store"
)

const maxUploadSize = 100 * 1024 * 1024 // 100MB

func (a *API) HandleUploadFile(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "file too large or invalid form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing file field")
		return
	}
	defer file.Close()

	f := &store.File{
		ID:          uuid.NewString(),
		Filename:    header.Filename,
		ContentType: header.Header.Get("Content-Type"),
	}
	if f.ContentType == "" {
		f.ContentType = "application/octet-stream"
	}

	if err := a.store.CreateFile(r.Context(), f, file); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save file")
		return
	}

	a.store.LogActivity(r.Context(), auth.UserID(r.Context()), "upload", "file", f.ID)
	writeJSON(w, http.StatusCreated, f)
}

func (a *API) HandleDownloadFile(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	file, meta, err := a.store.OpenFile(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to open file")
		return
	}
	defer file.Close()

	w.Header().Set("Content-Type", meta.ContentType)
	w.Header().Set("Content-Disposition", "attachment; filename="+meta.Filename)

	io.Copy(w, file)

	a.store.LogActivity(r.Context(), auth.UserID(r.Context()), "download", "file", id)
}

func (a *API) HandleListFiles(w http.ResponseWriter, r *http.Request) {
	files, err := a.store.ListFiles(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list files")
		return
	}

	writeJSON(w, http.StatusOK, files)
}

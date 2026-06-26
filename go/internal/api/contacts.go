package api

import (
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/weave-lab/interview-public/go/internal/auth"
	"github.com/weave-lab/interview-public/go/internal/store"
)

type listContactsResponse struct {
	Contacts      []store.Contact `json:"contacts"`
	NextPageToken string          `json:"next_page_token,omitempty"`
}

func (a *API) HandleListContacts(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}

	var cursor *store.PageToken
	if v := r.URL.Query().Get("page_token"); v != "" {
		cursor = decodePageToken(v)
		if cursor == nil {
			writeError(w, http.StatusBadRequest, "invalid page token")
			return
		}
	}

	contacts, err := a.store.ListContacts(r.Context(), limit+1, cursor)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list contacts")
		return
	}

	var nextToken string
	if len(contacts) > limit {
		last := contacts[limit-1]
		nextToken = encodePageToken(&store.PageToken{
			CreatedAt: last.CreatedAt,
			ID:        last.ID,
		})
		contacts = contacts[:limit]
	}

	writeJSON(w, http.StatusOK, listContactsResponse{
		Contacts:      contacts,
		NextPageToken: nextToken,
	})
}

func encodePageToken(t *store.PageToken) string {
	data, _ := json.Marshal(t)
	return base64.RawURLEncoding.EncodeToString(data)
}

func decodePageToken(s string) *store.PageToken {
	data, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil
	}
	var t store.PageToken
	if err := json.Unmarshal(data, &t); err != nil {
		return nil
	}
	if t.CreatedAt.IsZero() || t.ID == "" {
		return nil
	}
	return &t
}

func (a *API) HandleGetContact(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	contact, err := a.store.GetContact(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "contact not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get contact")
		return
	}

	writeJSON(w, http.StatusOK, contact)
}

func (a *API) HandleCreateContact(w http.ResponseWriter, r *http.Request) {
	var c store.Contact
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	c.ID = uuid.NewString()
	if err := a.store.CreateContact(r.Context(), &c); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create contact")
		return
	}

	a.store.LogActivity(r.Context(), auth.UserID(r.Context()), "create", "contact", c.ID)
	writeJSON(w, http.StatusCreated, c)
}

func (a *API) HandleUpdateContact(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var c store.Contact
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	c.ID = id
	if err := a.store.UpdateContact(r.Context(), &c); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "contact not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update contact")
		return
	}

	a.store.LogActivity(r.Context(), auth.UserID(r.Context()), "update", "contact", c.ID)
	writeJSON(w, http.StatusOK, c)
}

func (a *API) HandleDeleteContact(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := a.store.DeleteContact(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "contact not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete contact")
		return
	}

	a.store.LogActivity(r.Context(), auth.UserID(r.Context()), "delete", "contact", id)
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) HandleImportContacts(w http.ResponseWriter, r *http.Request) {
	var contacts []store.Contact
	if err := json.NewDecoder(r.Body).Decode(&contacts); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if len(contacts) > 10000 {
		writeError(w, http.StatusBadRequest, "maximum 10000 contacts per import")
		return
	}

	for i := range contacts {
		contacts[i].ID = uuid.NewString()
	}

	imported, err := a.store.ImportContacts(r.Context(), contacts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to import contacts")
		return
	}

	a.store.LogActivity(r.Context(), auth.UserID(r.Context()), "import", "contacts", "")
	writeJSON(w, http.StatusOK, map[string]int{"imported": imported})
}

func (a *API) HandleExportContacts(w http.ResponseWriter, r *http.Request) {
	contacts, err := a.store.ExportContacts(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to export contacts")
		return
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=contacts.csv")

	cw := csv.NewWriter(w)
	cw.Write([]string{"id", "first_name", "last_name", "email", "phone", "company", "created_at", "updated_at"})

	for _, c := range contacts {
		cw.Write([]string{
			c.ID,
			c.FirstName,
			c.LastName,
			c.Email,
			c.Phone,
			c.Company,
			c.CreatedAt.Format("2006-01-02T15:04:05Z"),
			c.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}
	cw.Flush()

	a.store.LogActivity(r.Context(), auth.UserID(r.Context()), "export", "contacts", "")
}

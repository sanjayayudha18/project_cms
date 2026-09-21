package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/cimb-niaga/cms/backend/internal/approval"
)

// writeUnexpectedError answers errors a master-data handler has no specific case for:
// an unroutable approval chain is the maker's setup problem (422, actionable); anything
// else is a logged 500 that never leaks the cause.
func writeUnexpectedError(w http.ResponseWriter, err error) {
	if errors.Is(err, approval.ErrChainIncomplete) {
		writeError(w, http.StatusUnprocessableEntity, "approval_chain_incomplete", "Akun Anda belum memiliki atasan penyetuju untuk perubahan ini. Hubungi administrator.")
		return
	}
	slog.Error("unexpected handler error", "error", err)
	writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
}

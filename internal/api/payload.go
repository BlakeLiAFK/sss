package api

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"sss/internal/config"
	"sss/internal/utils"
	"strconv"
	"strings"
)

type stagedBody struct{ *os.File }

func (b *stagedBody) Close() error {
	name := b.Name()
	err := b.File.Close()
	os.Remove(name)
	return err
}

// Stage authenticated bodies with bounded memory and verify before any mutation.
// The temporary file is outside the object store and cannot be collected by GC.
func verifyPayload(w http.ResponseWriter, r *http.Request) bool {
	limit := int64(5 << 30)
	for _, n := range []int64{config.Global.Storage.MaxUploadSize, config.Global.Storage.MaxObjectSize} {
		if n > 0 && n < limit {
			limit = n
		}
	}
	// Control documents have their own limit, independent of object byte limits.
	if r.Method != http.MethodPut || strings.HasPrefix(r.URL.Path, "/api/") || r.Header.Get("X-Amz-Copy-Source") != "" {
		limit = 10 << 20
	}
	if value := r.URL.Query().Get("X-Amz-Max-Content-Length"); value != "" {
		n, err := strconv.ParseInt(value, 10, 64)
		if err != nil || n <= 0 {
			utils.WriteError(w, utils.ErrInvalidArgument, 400, r.URL.Path)
			return false
		}
		if n < limit {
			limit = n
		}
	}
	file, err := os.CreateTemp("", "sss-payload-*")
	if err != nil {
		utils.WriteError(w, utils.ErrInternalError, 500, r.URL.Path)
		return false
	}
	body := &stagedBody{file}
	ok := false
	defer func() {
		if !ok {
			body.Close()
		}
	}()
	hash := sha256.New()
	var source io.Reader = http.NoBody
	if r.Body != nil {
		source = r.Body
		defer r.Body.Close()
	}
	size, err := io.Copy(io.MultiWriter(file, hash), io.LimitReader(source, limit+1))
	if size > limit {
		utils.WriteError(w, utils.ErrEntityTooLarge, 413, r.URL.Path)
		return false
	}
	if err != nil || (r.ContentLength >= 0 && r.ContentLength != size) {
		utils.WriteError(w, utils.ErrInvalidArgument, 400, r.URL.Path)
		return false
	}
	expected := r.Header.Get("X-Amz-Content-Sha256")
	if expected != "" && expected != "UNSIGNED-PAYLOAD" && expected != hex.EncodeToString(hash.Sum(nil)) {
		utils.WriteError(w, utils.ErrBadDigest, 400, r.URL.Path)
		return false
	}
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		utils.WriteError(w, utils.ErrInternalError, 500, r.URL.Path)
		return false
	}
	r.Body = body
	r.ContentLength = size
	ok = true
	return true
}

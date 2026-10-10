package api

import (
	"net/http"
	"sss/internal/storage"
	"sss/internal/utils"
	"strconv"
	"strings"
)

func matchesETag(header, etag string, exists bool) bool {
	for _, v := range strings.Split(header, ",") {
		v = strings.TrimSpace(v)
		if v == "*" {
			return exists
		}
		if exists && v == `"`+etag+`"` {
			return true
		}
	}
	return false
}
func (s *Server) checkWriteConditions(w http.ResponseWriter, r *http.Request, bucket, key string) bool {
	obj, err := s.metadata.GetObject(bucket, key)
	if err != nil {
		utils.WriteError(w, utils.ErrInternalError, 500, r.URL.Path)
		return false
	}
	etag := ""
	if obj != nil {
		etag = obj.ETag
	}
	if (r.Header.Get("If-Match") != "" && !matchesETag(r.Header.Get("If-Match"), etag, obj != nil)) || (r.Header.Get("If-None-Match") != "" && matchesETag(r.Header.Get("If-None-Match"), etag, obj != nil)) {
		utils.WriteError(w, utils.S3Error{Code: "PreconditionFailed", Message: "Precondition failed"}, 412, r.URL.Path)
		return false
	}
	return true
}
func checkReadConditions(w http.ResponseWriter, r *http.Request, obj *storage.Object) bool {
	w.Header().Set("ETag", `"`+obj.ETag+`"`)
	w.Header().Set("Last-Modified", obj.LastModified.UTC().Format(http.TimeFormat))
	if h := r.Header.Get("If-Match"); h != "" && !matchesETag(h, obj.ETag, true) {
		w.WriteHeader(412)
		return false
	}
	if h := r.Header.Get("If-None-Match"); h != "" {
		if matchesETag(h, obj.ETag, true) {
			w.WriteHeader(304)
			return false
		}
	} else if stamp, err := http.ParseTime(r.Header.Get("If-Modified-Since")); err == nil && !obj.LastModified.Truncate(1e9).After(stamp) {
		w.WriteHeader(304)
		return false
	}
	if r.Header.Get("If-Match") == "" {
		if stamp, err := http.ParseTime(r.Header.Get("If-Unmodified-Since")); err == nil && obj.LastModified.Truncate(1e9).After(stamp) {
			w.WriteHeader(412)
			return false
		}
	}
	return true
}
func parseRange(value string, size int64) (int64, int64, bool) {
	if size <= 0 || !strings.HasPrefix(value, "bytes=") {
		return 0, 0, false
	}
	p := strings.Split(strings.TrimPrefix(value, "bytes="), "-")
	if len(p) != 2 {
		return 0, 0, false
	}
	if p[0] == "" {
		n, err := strconv.ParseInt(p[1], 10, 64)
		if err != nil || n <= 0 {
			return 0, 0, false
		}
		if n > size {
			n = size
		}
		return size - n, size - 1, true
	}
	start, err := strconv.ParseInt(p[0], 10, 64)
	if err != nil || start < 0 || start >= size {
		return 0, 0, false
	}
	end := size - 1
	if p[1] != "" {
		end, err = strconv.ParseInt(p[1], 10, 64)
		if err != nil || end < start {
			return 0, 0, false
		}
		if end >= size {
			end = size - 1
		}
	}
	return start, end, true
}

// Copy's source conditions apply before publishing the destination. A matching
// If-Match takes precedence over If-Unmodified-Since, as in S3's contract.
func checkCopySourceConditions(w http.ResponseWriter, r *http.Request, obj *storage.Object) bool {
	fail := func() bool {
		utils.WriteError(w, utils.S3Error{Code: "PreconditionFailed", Message: "Copy source precondition failed"}, 412, r.URL.Path)
		return false
	}
	match := r.Header.Get("X-Amz-Copy-Source-If-Match")
	if match != "" && !matchesETag(match, obj.ETag, true) {
		return fail()
	}
	if h := r.Header.Get("X-Amz-Copy-Source-If-None-Match"); h != "" && matchesETag(h, obj.ETag, true) {
		return fail()
	}
	for _, name := range []string{"X-Amz-Copy-Source-If-Modified-Since", "X-Amz-Copy-Source-If-Unmodified-Since"} {
		value := r.Header.Get(name)
		if value == "" {
			continue
		}
		stamp, err := http.ParseTime(value)
		if err != nil {
			utils.WriteError(w, utils.ErrInvalidArgument, 400, r.URL.Path)
			return false
		}
		after := obj.LastModified.Truncate(1e9).After(stamp)
		if name == "X-Amz-Copy-Source-If-Modified-Since" && !after {
			return fail()
		}
		if name == "X-Amz-Copy-Source-If-Unmodified-Since" && match == "" && after {
			return fail()
		}
	}
	return true
}

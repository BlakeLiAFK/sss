package utils

import (
	"fmt"
	"io"
	"net/http"
	"os"
)

type temporaryBody struct{ *os.File }

func (b *temporaryBody) Close() error { err := b.File.Close(); os.Remove(b.Name()); return err }

// StageRequestBody bounds disk and memory without holding a storage lock.
func StageRequestBody(r *http.Request, limit int64) error {
	file, err := os.CreateTemp("", "sss-control-*")
	if err != nil {
		return err
	}
	body := &temporaryBody{file}
	good := false
	defer func() {
		if !good {
			body.Close()
		}
	}()
	var source io.Reader = http.NoBody
	if r.Body != nil {
		source = r.Body
		defer r.Body.Close()
	}
	size, err := io.Copy(file, io.LimitReader(source, limit+1))
	if err != nil {
		return err
	}
	if size > limit {
		return fmt.Errorf("request body too large")
	}
	if r.ContentLength >= 0 && size != r.ContentLength {
		return fmt.Errorf("incomplete request body")
	}
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	r.Body = body
	r.ContentLength = size
	good = true
	return nil
}

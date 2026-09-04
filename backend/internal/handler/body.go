package handler

import (
	"bytes"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"
)

func readBody(w http.ResponseWriter, r *http.Request) ([]byte, bool, error) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	b, err := io.ReadAll(r.Body)
	if err != nil {
		var mb *http.MaxBytesError
		if errors.As(err, &mb) {
			return nil, false, errInvalidJSON
		}
		return nil, false, errInvalidJSON
	}
	if len(bytes.TrimSpace(b)) == 0 {
		return nil, true, nil
	}
	ct := r.Header.Get("Content-Type")
	if ct == "" {
		return nil, false, errInvalidJSON
	}
	mt, _, err := mime.ParseMediaType(ct)
	if err != nil {
		return nil, false, errInvalidJSON
	}
	if mt != "application/json" && !strings.HasSuffix(mt, "+json") {
		return nil, false, errInvalidJSON
	}
	return b, false, nil
}

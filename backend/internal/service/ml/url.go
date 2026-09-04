package ml

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

var (
	errEmptyBaseURL     = errors.New("ml base url is empty")
	errBaseURLNoHost    = errors.New("ml base url has no host")
	errBaseURLBadScheme = errors.New("ml base url scheme is not http or https")
	errBaseURLExtraPart = errors.New("ml base url must not contain query or fragment")
)

// validateBaseURL проверяет адрес ML: только http/https, без query и фрагмента.
func validateBaseURL(baseURL string) error {
	if baseURL == "" {
		return errEmptyBaseURL
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return fmt.Errorf("parse ml base url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errBaseURLBadScheme
	}
	if u.Host == "" {
		return errBaseURLNoHost
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return errBaseURLExtraPart
	}
	return nil
}

// joinURL соединяет базовый адрес с путём, убирая лишний слэш.
func joinURL(baseURL, path string) string {
	return strings.TrimSuffix(baseURL, "/") + path
}

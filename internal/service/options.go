package service

import (
	"errors"
	"time"
)

type Option func(*ShortenerService)

func WithShortCodeLength(length int) Option {
	return func(s *ShortenerService) {
		s.codeLen = length
	}
}

func WithDefaultTTL(ttl time.Duration) Option {
	return func(s *ShortenerService) {
		s.defaultTTL = ttl
	}
}

func WithBaseURL(baseURL string) Option {
	return func(s *ShortenerService) {
		s.baseURL = baseURL
	}
}

func WithMaxRetries(retries int) Option {
	return func(s *ShortenerService) {
		s.maxRetries = retries
	}
}

func (s *ShortenerService) validate() error {
	if s.codeLen <= 0 {
		return errors.New("invalid code length: must be > 0")
	}
	if s.defaultTTL <= 0 {
		return errors.New("invalid default ttl: must be > 0")
	}
	if s.maxRetries <= 0 {
		return errors.New("invalid max retries: must be > 0")
	}
	if s.baseURL == "" {
		return errors.New("invalid base url: must be non-nil")
	}
	return nil
}

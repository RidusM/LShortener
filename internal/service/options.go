package service

import (
	"time"
)

type Option func(*ShortenerService)

func ShortCodeLength(length int) Option {
	return func(s *ShortenerService) {
		if length > 0 {
			s.codeLen = length
		}
	}
}

func DefaultTTL(ttl time.Duration) Option {
	return func(s *ShortenerService) {
		s.defaultTTL = ttl
	}
}

func BaseURL(baseURL string) Option {
	return func(s *ShortenerService) {
		if baseURL != "" {
			s.baseURL = baseURL
		}
	}
}

func MaxRetries(retries int) Option {
	return func(s *ShortenerService) {
		if retries > 0 {
			s.maxRetries = retries
		}
	}
}

func QueryLimit(limit uint64) Option {
	return func(s *ShortenerService) {
		if limit > 0 {
			s.queryLimit = limit
		}
	}
}

func RetryDelay(delay time.Duration) Option {
	return func(s *ShortenerService) {
		if delay > 0 {
			s.retryDelay = delay
		}
	}
}

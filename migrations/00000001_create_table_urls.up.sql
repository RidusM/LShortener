CREATE TABLE IF NOT EXISTS urls (
    id UUID PRIMARY KEY,
    short_code VARCHAR(12) UNIQUE NOT NULL,
    original_url TEXT NOT NULL,
    custom_alias VARCHAR(50) UNIQUE,
    expires_at TIMESTAMP,
    is_active BOOLEAN DEFAULT TRUE,
    click_count BIGINT DEFAULT 0,

    CHECK (LENGTH(short_code) >= 4 AND LENGTH(short_code) <= 12)
);

CREATE INDEX idx_urls_short_code ON urls(short_code);
CREATE INDEX idx_urls_custom_alias ON urls(custom_alias) WHERE custom_alias IS NOT NULL;
CREATE INDEX idx_urls_expires_at ON urls(expires_at) WHERE expires_at IS NOT NULL;

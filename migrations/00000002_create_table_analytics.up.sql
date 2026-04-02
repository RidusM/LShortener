CREATE TABLE IF NOT EXISTS analytics (
    id UUID PRIMARY KEY,
    url_id UUID NOT NULL REFERENCES urls(id) ON DELETE CASCADE,
    user_agent TEXT,
    ip_address VARCHAR(45),
    referer TEXT,
    clicked_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_analytics_url_id ON analytics(url_id);
CREATE INDEX idx_analytics_clicked_at ON analytics(clicked_at DESC);

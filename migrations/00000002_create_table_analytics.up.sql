CREATE TABLE IF NOT EXISTS analytics (
    id UUID PRIMARY KEY,
    url_id UUID NOT NULL REFERENCES urls(id) ON DELETE CASCADE,
    user_agent TEXT NOT NULL,
    ip_address INET NOT NULL,
    referer TEXT,
    clicked_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_analytics_url_id ON analytics (url_id);
CREATE INDEX IF NOT EXISTS idx_analytics_clicked_at ON analytics (clicked_at);
CREATE INDEX IF NOT EXISTS idx_analytics_url_id_clicked_at ON analytics (url_id, clicked_at DESC);

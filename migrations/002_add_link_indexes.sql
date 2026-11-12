
CREATE INDEX IF NOT EXISTS idx_links_code
    ON links(code);

CREATE INDEX IF NOT EXISTS idx_links_created_at
    ON links(created_at);

CREATE INDEX IF NOT EXISTS idx_links_active
    ON links(active);


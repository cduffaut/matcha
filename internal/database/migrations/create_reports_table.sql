-- Nouvelle migration: create_reports_table.sql
-- Table pour les signalements d'utilisateurs

CREATE TABLE IF NOT EXISTS user_reports (
    id SERIAL PRIMARY KEY,
    reporter_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    reported_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    reason TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    is_processed BOOLEAN DEFAULT FALSE,
    processed_at TIMESTAMP,
    admin_comment TEXT,
    UNIQUE (reporter_id, reported_id)
);

-- Index pour améliorer les performances
CREATE INDEX IF NOT EXISTS idx_user_reports_reporter_id ON user_reports(reporter_id);
CREATE INDEX IF NOT EXISTS idx_user_reports_reported_id ON user_reports(reported_id);
CREATE INDEX IF NOT EXISTS idx_user_reports_is_processed ON user_reports(is_processed);
CREATE INDEX IF NOT EXISTS idx_user_reports_created_at ON user_reports(created_at);

-- Contraintes
ALTER TABLE user_reports ADD CONSTRAINT IF NOT EXISTS chk_no_self_report 
CHECK (reporter_id != reported_id);

ALTER TABLE user_reports ADD CONSTRAINT IF NOT EXISTS chk_reason_not_empty 
CHECK (length(trim(reason)) >= 10);
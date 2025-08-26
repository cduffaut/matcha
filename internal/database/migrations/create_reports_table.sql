CREATE TABLE IF NOT EXISTS user_reports (
	id SERIAL PRIMARY KEY,
	reporter_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	reported_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	reason TEXT NOT NULL CHECK (length(trim(reason)) >= 10),
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	is_processed BOOLEAN DEFAULT FALSE,
	processed_at TIMESTAMP,
	admin_comment TEXT,
	UNIQUE (reporter_id, reported_id),
	CHECK (reporter_id != reported_id)
);

CREATE INDEX IF NOT EXISTS idx_user_reports_reporter_id ON user_reports(reporter_id);
CREATE INDEX IF NOT EXISTS idx_user_reports_reported_id ON user_reports(reported_id);
CREATE INDEX IF NOT EXISTS idx_user_reports_is_processed ON user_reports(is_processed);
CREATE INDEX IF NOT EXISTS idx_user_reports_created_at ON user_reports(created_at);

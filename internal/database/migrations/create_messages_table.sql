-- Migration pour créer la table des messages
CREATE TABLE IF NOT EXISTS messages (
    id SERIAL PRIMARY KEY,
    sender_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    recipient_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    is_read BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Index pour améliorer les performances
CREATE INDEX IF NOT EXISTS idx_messages_sender_recipient ON messages(sender_id, recipient_id);
CREATE INDEX IF NOT EXISTS idx_messages_recipient_sender ON messages(recipient_id, sender_id);
CREATE INDEX IF NOT EXISTS idx_messages_recipient_unread ON messages(recipient_id) WHERE is_read = FALSE;
CREATE INDEX IF NOT EXISTS idx_messages_created_at ON messages(created_at);

-- Index pour les conversations (bidirectionnel)
CREATE INDEX IF NOT EXISTS idx_messages_conversation_asc ON messages(sender_id, recipient_id, created_at);
CREATE INDEX IF NOT EXISTS idx_messages_conversation_desc ON messages(recipient_id, sender_id, created_at);

-- Contraintes (avec vérification d'existence)
DO $$
BEGIN
    -- Contrainte pour éviter les messages vides
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.table_constraints 
        WHERE constraint_name = 'chk_message_content_not_empty'
        AND table_name = 'messages'
    ) THEN
        ALTER TABLE messages ADD CONSTRAINT chk_message_content_not_empty 
        CHECK (length(trim(content)) > 0);
    END IF;
    
    -- Contrainte pour éviter les messages à soi-même
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.table_constraints 
        WHERE constraint_name = 'chk_no_self_message'
        AND table_name = 'messages'
    ) THEN
        ALTER TABLE messages ADD CONSTRAINT chk_no_self_message 
        CHECK (sender_id != recipient_id);
    END IF;
END $$;
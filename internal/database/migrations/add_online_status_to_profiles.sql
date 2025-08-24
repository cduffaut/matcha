-- Ajouter les colonnes pour le statut en ligne et la dernière connexion
-- (si elles n'existent pas déjà)
DO $$ 
BEGIN
    -- Ajouter is_online si elle n'existe pas
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name = 'user_profiles' AND column_name = 'is_online'
    ) THEN
        ALTER TABLE user_profiles ADD COLUMN is_online BOOLEAN DEFAULT FALSE;
    END IF;
    
    -- Ajouter last_connection si elle n'existe pas
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name = 'user_profiles' AND column_name = 'last_connection'
    ) THEN
        ALTER TABLE user_profiles ADD COLUMN last_connection TIMESTAMP;
    END IF;
END $$;

-- Index pour améliorer les performances
CREATE INDEX IF NOT EXISTS idx_user_profiles_is_online ON user_profiles(is_online);
CREATE INDEX IF NOT EXISTS idx_user_profiles_last_connection ON user_profiles(last_connection);
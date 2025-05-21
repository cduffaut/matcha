ALTER TABLE user_profiles ADD COLUMN birth_date DATE;

-- Mise à jour des profils existants avec une date de naissance par défaut
-- (25 ans par défaut, à ajuster selon vos besoins)
UPDATE user_profiles SET birth_date = CURRENT_DATE - INTERVAL '25 years';
DO $$
BEGIN
	IF NOT EXISTS (
		SELECT 1
		FROM information_schema.columns
		WHERE table_name='user_profiles'
		  AND column_name='birth_date'
	) THEN
		ALTER TABLE user_profiles ADD COLUMN birth_date DATE;
		UPDATE user_profiles SET birth_date = CURRENT_DATE - INTERVAL '25 years';
	END IF;
END
$$;
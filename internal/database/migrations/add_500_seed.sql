DO $$
DECLARE
  existing_users INT;
BEGIN
  SELECT COUNT(*) INTO existing_users FROM users;

  IF existing_users = 0 THEN
    RAISE NOTICE 'Seeding from CSV since DB is empty';

    -- 1) Tags (fixes, avec id)
    COPY tags(id, name)
    FROM '/mock/tags.csv'
    DELIMITER ',' CSV HEADER;

    -- 2) Users (ids générés séquentiels 1..500 en base vide)
    COPY users(username, email, first_name, last_name, password, is_verified)
    FROM '/mock/users.csv'
    DELIMITER ',' CSV HEADER;

    -- 3) Profils (référence user_id 1..500)
    COPY user_profiles(user_id, gender, sexual_preferences, biography, fame_rating,
                       latitude, longitude, location_name, birth_date, is_online, last_connection)
    FROM '/mock/user_profiles.csv'
    DELIMITER ',' CSV HEADER;

    -- 4) Photos (id SERIAL auto, on ne fournit pas id)
    COPY user_photos(user_id, file_path, is_profile)
    FROM '/mock/user_photos.csv'
    DELIMITER ',' CSV HEADER;

    -- 5) Liaisons tags
    COPY user_tags(user_id, tag_id)
    FROM '/mock/user_tags.csv'
    DELIMITER ',' CSV HEADER;

    -- Ajuster les séquences si besoin
    PERFORM setval(pg_get_serial_sequence('tags','id'), (SELECT COALESCE(MAX(id),1) FROM tags), TRUE);
    PERFORM setval(pg_get_serial_sequence('user_photos','id'), (SELECT COALESCE(MAX(id),1) FROM user_photos), TRUE);
    PERFORM setval(pg_get_serial_sequence('users','id'), (SELECT COALESCE(MAX(id),1) FROM users), TRUE);

  ELSE
    RAISE NOTICE 'Skip seed: users table is not empty (count = %)', existing_users;
  END IF;
END $$;
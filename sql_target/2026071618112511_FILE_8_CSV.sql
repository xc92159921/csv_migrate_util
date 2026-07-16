DO $$
BEGIN

    BEGIN
        COPY file_8 (id,name8)
        FROM '/data/file_8.csv' 
        DELIMITER ',' CSV HEADER;
    EXCEPTION 
        WHEN undefined_file THEN
            RAISE NOTICE 'Файл file_8.csv не найден, пропускаем импорт данных.';
    END;


END $$;

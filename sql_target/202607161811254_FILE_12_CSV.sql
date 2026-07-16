DO $$
BEGIN

    BEGIN
        COPY file_12 (id,name12)
        FROM '/data/file_12.csv' 
        DELIMITER ',' CSV HEADER;
    EXCEPTION 
        WHEN undefined_file THEN
            RAISE NOTICE 'Файл file_12.csv не найден, пропускаем импорт данных.';
    END;


END $$;

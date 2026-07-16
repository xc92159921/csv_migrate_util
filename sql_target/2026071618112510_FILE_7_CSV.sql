DO $$
BEGIN

    BEGIN
        COPY file_7 (id,name7)
        FROM '/data/file_7.csv' 
        DELIMITER ',' CSV HEADER;
    EXCEPTION 
        WHEN undefined_file THEN
            RAISE NOTICE 'Файл file_7.csv не найден, пропускаем импорт данных.';
    END;


END $$;

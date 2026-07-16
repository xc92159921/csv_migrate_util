DO $$
BEGIN

    BEGIN
        COPY file_10 (id,name10)
        FROM '/data/file_10.csv' 
        DELIMITER ',' CSV HEADER;
    EXCEPTION 
        WHEN undefined_file THEN
            RAISE NOTICE 'Файл file_10.csv не найден, пропускаем импорт данных.';
    END;


END $$;

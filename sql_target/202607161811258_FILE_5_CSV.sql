DO $$
BEGIN

    BEGIN
        COPY file_5 (id,name5)
        FROM '/data/file_5.csv' 
        DELIMITER ',' CSV HEADER;
    EXCEPTION 
        WHEN undefined_file THEN
            RAISE NOTICE 'Файл file_5.csv не найден, пропускаем импорт данных.';
    END;


END $$;

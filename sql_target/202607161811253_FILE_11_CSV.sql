DO $$
BEGIN

    BEGIN
        COPY file_11 (id,name11)
        FROM '/data/file_11.csv' 
        DELIMITER ',' CSV HEADER;
    EXCEPTION 
        WHEN undefined_file THEN
            RAISE NOTICE 'Файл file_11.csv не найден, пропускаем импорт данных.';
    END;


END $$;

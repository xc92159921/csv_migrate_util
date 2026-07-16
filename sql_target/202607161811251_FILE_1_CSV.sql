DO $$
BEGIN

    BEGIN
        COPY file_1 (id,name1)
        FROM '/data/file_1.csv' 
        DELIMITER ',' CSV HEADER;
    EXCEPTION 
        WHEN undefined_file THEN
            RAISE NOTICE 'Файл file_1.csv не найден, пропускаем импорт данных.';
    END;


END $$;

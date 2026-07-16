DO $$
BEGIN

    BEGIN
        COPY file (id,name1)
        FROM '/data/1.file.csv' 
        DELIMITER ',' CSV HEADER;
    EXCEPTION 
        WHEN undefined_file THEN
            RAISE NOTICE 'Файл 1.file.csv не найден, пропускаем импорт данных.';
    END;


END $$;

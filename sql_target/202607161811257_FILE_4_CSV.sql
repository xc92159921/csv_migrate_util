DO $$
BEGIN

    BEGIN
        COPY file_4 (id,name4)
        FROM '/data/file_4.csv' 
        DELIMITER ',' CSV HEADER;
    EXCEPTION 
        WHEN undefined_file THEN
            RAISE NOTICE 'Файл file_4.csv не найден, пропускаем импорт данных.';
    END;


END $$;

DO $$
BEGIN

    BEGIN
        COPY file_3 (id,name3)
        FROM '/data/file_3.csv' 
        DELIMITER ',' CSV HEADER;
    EXCEPTION 
        WHEN undefined_file THEN
            RAISE NOTICE 'Файл file_3.csv не найден, пропускаем импорт данных.';
    END;


END $$;

DO $$
BEGIN

    BEGIN
        COPY file_9 (id,name9)
        FROM '/data/file_9.csv' 
        DELIMITER ',' CSV HEADER;
    EXCEPTION 
        WHEN undefined_file THEN
            RAISE NOTICE 'Файл file_9.csv не найден, пропускаем импорт данных.';
    END;


END $$;

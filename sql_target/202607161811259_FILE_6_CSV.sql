DO $$
BEGIN

    BEGIN
        COPY file_6 (id,name6)
        FROM '/data/file_6.csv' 
        DELIMITER ',' CSV HEADER;
    EXCEPTION 
        WHEN undefined_file THEN
            RAISE NOTICE 'Файл file_6.csv не найден, пропускаем импорт данных.';
    END;


END $$;

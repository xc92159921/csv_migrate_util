DO $$
BEGIN

    BEGIN
        COPY file_2 (id,name2)
        FROM '/data/file_2.csv' 
        DELIMITER ',' CSV HEADER;
    EXCEPTION 
        WHEN undefined_file THEN
            RAISE NOTICE 'Файл file_2.csv не найден, пропускаем импорт данных.';
    END;


END $$;

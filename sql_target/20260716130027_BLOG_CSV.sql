DO $$
BEGIN

    BEGIN
        COPY blogs (id,title,description,preview,preview_small,show_on_main,url,article,views,user_blogs)
        FROM '/data/blog.csv' 
        DELIMITER ',' CSV HEADER;
    EXCEPTION 
        WHEN undefined_file THEN
            RAISE NOTICE 'Файл blog.csv не найден, пропускаем импорт данных.';
    END;


END $$;
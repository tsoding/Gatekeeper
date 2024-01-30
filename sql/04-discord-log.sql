CREATE TABLE Discord_Log(
    message_id varchar(32),
    user_id varchar(32),
    user_name varchar(32),
    posted_at timestamp DEFAULT now(),
    text varchar(2000)
);

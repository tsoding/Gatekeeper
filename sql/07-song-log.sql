CREATE TABLE Song_Log(
    artist varchar(256),
    title varchar(256),
    startedAt timestamp DEFAULT now()
)

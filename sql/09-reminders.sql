CREATE TABLE Reminders(
    id bigserial primary key,
    discord_user_id varchar(32),
    message varchar(256),
    remind_at timestamptz NOT NULL
)

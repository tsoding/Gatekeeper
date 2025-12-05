CREATE TABLE Reminders(
    id bigserial primary key,
    user_id varchar(32),
    message varchar(256),
    remind_at timestamptz NOT NULL
)

CREATE TABLE Ed_State(
    -- NOTE: user_id is a string that uniquely identifies the user across environments.
    -- On Twitch it's the nickname (which is between 4-25 characters + "twitch#" prefix).
    -- On Discord it's the user id (which is a Snowflake ID which is a 64bit integer the maximium
    -- value of which in decimal is 20 characters long + "discord#" prefix).
    -- So we set the size of the user ID as 32 just in case to accomodate both of the ids.
    user_id varchar(32),
    -- NOTE: the size of the buffer is based on EdLineCountLimit and EdLineSizeLimit constants.
    -- The formula is 2*EdLineCountLimit*EdLineSizeLimit (the 2 is to accomodate the newlines)
    -- If the constants are modified, this size should be adjusted as well.
    buffer varchar(2*5*100),
    cur int,
    mode int,
    UNIQUE(user_id)
);

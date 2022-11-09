CREATE TABLE Carrotson_Branches (
    context varchar(8),
    follows char,
    frequency bigint,
    UNIQUE(context, follows)
);

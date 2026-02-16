create table if not exists Grok(
	yes boolean,
	word varchar(256),
	count bigint,
	unique(yes, word)
);

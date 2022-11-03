CREATE TABLE TrustLog (
    trusterId varchar(32),
    trusteeId varchar(32),
    trustedAt timestamp DEFAULT now()
)

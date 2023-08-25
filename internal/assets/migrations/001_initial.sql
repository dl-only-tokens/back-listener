-- +migrate Up
CREATE TABLE transactions (
    payment_id VARCHAR(73),
    sender CHAR(42),
    recipient CHAR(42),
    tx_hash_to VARCHAR(100) ,
    tx_hash_from VARCHAR(100) ,
    network_to  VARCHAR(10),
    network_from  VARCHAR(10),
    value_to numeric,
    timestamp_to timestamp,
    currency  VARCHAR(10),
    PRIMARY KEY (payment_id)
);




-- +migrate Down
DROP TABLE transactions;


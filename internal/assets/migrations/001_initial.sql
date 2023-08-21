-- +migrate Up
CREATE TABLE transactions (
    payment_id VARCHAR(73),
    recipient VARCHAR(42),
    tx_hash_to VARCHAR(100) ,
    tx_hash_from VARCHAR(100) ,
    network_to  VARCHAR(10),
    network_from  VARCHAR(10),
    PRIMARY KEY (tx_hash_from,  network_from)
);




-- +migrate Down
DROP TABLE transactions;


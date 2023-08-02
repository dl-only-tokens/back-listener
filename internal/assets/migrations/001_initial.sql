-- +migrate Up
CREATE TABLE transactions (
    id SERIAL PRIMARY KEY,
    payment_id char(73),
    recipient char(42),
    tx_hash char(64),
    network  varchar(10)
);


-- +migrate Down
DROP TABLE transactions
-- +goose Up
CREATE TABLE btc.tx_inputs (
  id             SERIAL PRIMARY KEY,
	tx_id          INTEGER NOT NULL REFERENCES btc.transaction_cids (id) ON DELETE CASCADE DEFERRABLE INITIALLY DEFERRED,
	index          INTEGER NOT NULL,
	tx_witness     BYTEA[],
	sequence       INTEGER NOT NULL,
	script         BYTEA NOT NULL,
	outpoint_hash  VARCHAR(66) NOT NULL,
	outpoint_index INTEGER NOT NULL,
	UNIQUE (tx_id, index)
);

-- +goose Down
DROP TABLE btc.tx_inputs;
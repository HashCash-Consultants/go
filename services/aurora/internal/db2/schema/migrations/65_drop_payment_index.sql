-- +migrate Up

-- index_history_operations_on_is_payment was added in migration 64 but it turns out
-- the index was not necessary, see https://github.com/shantanu-hashcash/go/issues/5059
DROP INDEX IF EXISTS "index_history_operations_on_is_payment";

-- +migrate Down

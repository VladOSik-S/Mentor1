-- +goose Up
-- +goose StatementBegin
CREATE SEQUENCE IF NOT EXISTS users_id_seq
   START WITH 1
   INCREMENT BY 1
   MINVALUE 1
   MAXVALUE 2147483647
   CACHE 1;

CREATE TABLE users (
   id integer DEFAULT nextval('users_id_seq'::regclass) NOT NULL,
   telegram_id character varying(255) NOT NULL,
   name character varying(255) NOT NULL,
   status character varying(255) DEFAULT 'Новичок'::character varying,
   avatar_url text,
   created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
   role character varying(50) DEFAULT 'student'::character varying,
   CONSTRAINT users_pkey PRIMARY KEY (id)
);

CREATE UNIQUE INDEX users_telegram_id_key ON users (telegram_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS users CASCADE;
DROP SEQUENCE IF EXISTS users_id_seq;
-- +goose StatementEnd

-- +goose Up
-- +goose StatementBegin
CREATE SEQUENCE IF NOT EXISTS achievements_id_seq
   START WITH 1
   INCREMENT BY 1
   MINVALUE 1
   MAXVALUE 2147483647
   CACHE 1;

CREATE TABLE achievements (
   id integer DEFAULT nextval('achievements_id_seq'::regclass) NOT NULL,
   name character varying(255) NOT NULL,
   description text NOT NULL,
   icon character varying(10) NOT NULL,
   created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
   CONSTRAINT achievements_pkey PRIMARY KEY (id)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS achievements CASCADE;
DROP SEQUENCE IF EXISTS achievements_id_seq;
-- +goose StatementEnd

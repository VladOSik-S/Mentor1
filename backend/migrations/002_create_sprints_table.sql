-- +goose Up
-- +goose StatementBegin
CREATE SEQUENCE IF NOT EXISTS sprints_id_seq
   START WITH 1
   INCREMENT BY 1
   MINVALUE 1
   MAXVALUE 2147483647
   CACHE 1;

CREATE TABLE sprints (
   id integer DEFAULT nextval('sprints_id_seq'::regclass) NOT NULL,
   name character varying(255) NOT NULL,
   description text,
   duration_days integer DEFAULT 14,
   order_index integer NOT NULL,
   created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
   CONSTRAINT sprints_pkey PRIMARY KEY (id)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS sprints CASCADE;
DROP SEQUENCE IF EXISTS sprints_id_seq;
-- +goose StatementEnd

-- +goose Up
-- +goose StatementBegin
CREATE SEQUENCE IF NOT EXISTS notes_id_seq
   START WITH 1
   INCREMENT BY 1
   MINVALUE 1
   MAXVALUE 2147483647
   CACHE 1;

CREATE TABLE notes (
   id integer DEFAULT nextval('notes_id_seq'::regclass) NOT NULL,
   title character varying(255) NOT NULL,
   content text NOT NULL,
   tags text[] DEFAULT '{}'::text[],
   created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
   updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
   is_public boolean DEFAULT false,
   saved_to_migration boolean DEFAULT false,
   preview text,
   CONSTRAINT notes_pkey PRIMARY KEY (id)
);

CREATE INDEX idx_notes_tags ON notes (tags);
CREATE UNIQUE INDEX idx_notes_title ON notes (title);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS notes CASCADE;
DROP SEQUENCE IF EXISTS notes_id_seq;
-- +goose StatementEnd

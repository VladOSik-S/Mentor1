-- +goose Up
-- +goose StatementBegin
CREATE SEQUENCE IF NOT EXISTS tasks_id_seq
   START WITH 1
   INCREMENT BY 1
   MINVALUE 1
   MAXVALUE 2147483647
   CACHE 1;

CREATE TABLE tasks (
   id integer DEFAULT nextval('tasks_id_seq'::regclass) NOT NULL,
   sprint_id integer,
   name character varying(255) NOT NULL,
   description text,
   type character varying(20) NOT NULL,
   content_url text,
   time_minutes integer NOT NULL,
   order_index integer NOT NULL,
   created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
   content text,
   CONSTRAINT tasks_pkey PRIMARY KEY (id),
   CONSTRAINT tasks_sprint_id_fkey FOREIGN KEY (sprint_id) 
      REFERENCES sprints (id) ON UPDATE NO ACTION ON DELETE CASCADE
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS tasks CASCADE;
DROP SEQUENCE IF EXISTS tasks_id_seq;
-- +goose StatementEnd

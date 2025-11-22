-- +goose Up
-- +goose StatementBegin
CREATE SEQUENCE IF NOT EXISTS task_solutions_id_seq
   START WITH 1
   INCREMENT BY 1
   MINVALUE 1
   MAXVALUE 2147483647
   CACHE 1;

CREATE TABLE task_solutions (
   id integer DEFAULT nextval('task_solutions_id_seq'::regclass) NOT NULL,
   task_id integer,
   user_id integer,
   content text NOT NULL,
   status character varying(20) DEFAULT 'pending'::character varying,
   created_at timestamp without time zone DEFAULT now(),
   reviewed_at timestamp without time zone,
   reviewer_id integer,
   comment text,
   CONSTRAINT task_solutions_pkey PRIMARY KEY (id),
   CONSTRAINT task_solutions_task_id_fkey FOREIGN KEY (task_id) 
      REFERENCES tasks (id) ON UPDATE NO ACTION ON DELETE CASCADE,
   CONSTRAINT task_solutions_user_id_fkey FOREIGN KEY (user_id) 
      REFERENCES users (id) ON UPDATE NO ACTION ON DELETE CASCADE,
   CONSTRAINT task_solutions_reviewer_id_fkey FOREIGN KEY (reviewer_id) 
      REFERENCES users (id) ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS task_solutions CASCADE;
DROP SEQUENCE IF EXISTS task_solutions_id_seq;
-- +goose StatementEnd

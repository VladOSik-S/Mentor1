-- migrations/009_user_question_tags.sql
-- +goose Up
-- +goose StatementBegin
CREATE TABLE user_question_tags (
   user_id integer NOT NULL,
   tag character varying(255) NOT NULL,
   assigned_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
   assigned_by integer, -- кто назначил (admin_id)
   CONSTRAINT user_question_tags_pkey PRIMARY KEY (user_id, tag),
   CONSTRAINT user_question_tags_user_id_fkey FOREIGN KEY (user_id) 
      REFERENCES users (id) ON UPDATE NO ACTION ON DELETE CASCADE,
   CONSTRAINT user_question_tags_assigned_by_fkey FOREIGN KEY (assigned_by) 
      REFERENCES users (id) ON UPDATE NO ACTION ON DELETE SET NULL
);

CREATE INDEX idx_user_question_tags_user ON user_question_tags (user_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS user_question_tags CASCADE;
-- +goose StatementEnd

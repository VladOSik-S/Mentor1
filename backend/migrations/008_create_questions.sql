-- +goose Up
-- +goose StatementBegin
CREATE SEQUENCE IF NOT EXISTS questions_id_seq
   START WITH 1
   INCREMENT BY 1
   MINVALUE 1
   MAXVALUE 2147483647
   CACHE 1;

CREATE TABLE questions (
   id integer DEFAULT nextval('questions_id_seq'::regclass) NOT NULL,
   question text NOT NULL,
   answer text NOT NULL,
   tags text[] DEFAULT '{}'::text[],
   difficulty integer DEFAULT 1, -- 1-3 (легкий, средний, сложный)
   created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
   updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
   CONSTRAINT questions_pkey PRIMARY KEY (id)
);

CREATE INDEX idx_questions_tags ON questions (tags);
CREATE INDEX idx_questions_difficulty ON questions (difficulty);

-- User Questions (для интервального повторения)
CREATE TABLE user_questions (
   user_id integer NOT NULL,
   question_id integer NOT NULL,
   easiness_factor real DEFAULT 2.5, -- SM-2 алгоритм
   interval integer DEFAULT 0, -- дни до следующего повторения
   repetitions integer DEFAULT 0, -- количество успешных повторений
   next_review_date timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
   last_reviewed_at timestamp without time zone,
   status character varying(20) DEFAULT 'new'::character varying, -- new, learning, reviewing, mastered
   CONSTRAINT user_questions_pkey PRIMARY KEY (user_id, question_id),
   CONSTRAINT user_questions_user_id_fkey FOREIGN KEY (user_id) 
      REFERENCES users (id) ON UPDATE NO ACTION ON DELETE CASCADE,
   CONSTRAINT user_questions_question_id_fkey FOREIGN KEY (question_id) 
      REFERENCES questions (id) ON UPDATE NO ACTION ON DELETE CASCADE
);

CREATE INDEX idx_user_questions_status ON user_questions (user_id, status);
CREATE INDEX idx_user_questions_next_review ON user_questions (user_id, next_review_date);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS user_questions CASCADE;
DROP TABLE IF EXISTS questions CASCADE;
DROP SEQUENCE IF EXISTS questions_id_seq;
-- +goose StatementEnd

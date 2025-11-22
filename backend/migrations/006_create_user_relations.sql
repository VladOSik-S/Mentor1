-- +goose Up
-- +goose StatementBegin
-- User Settings
CREATE TABLE user_settings (
   user_id integer NOT NULL,
   theme character varying(10) DEFAULT 'dark'::character varying,
   notifications_enabled boolean DEFAULT true,
   CONSTRAINT user_settings_pkey PRIMARY KEY (user_id),
   CONSTRAINT user_settings_user_id_fkey FOREIGN KEY (user_id) 
      REFERENCES users (id) ON UPDATE NO ACTION ON DELETE CASCADE
);

-- User Sprints
CREATE TABLE user_sprints (
   user_id integer NOT NULL,
   sprint_id integer NOT NULL,
   status character varying(20) DEFAULT 'locked'::character varying,
   started_at timestamp without time zone,
   completed_at timestamp without time zone,
   CONSTRAINT user_sprints_pkey PRIMARY KEY (user_id, sprint_id),
   CONSTRAINT user_sprints_user_id_fkey FOREIGN KEY (user_id) 
      REFERENCES users (id) ON UPDATE NO ACTION ON DELETE CASCADE,
   CONSTRAINT user_sprints_sprint_id_fkey FOREIGN KEY (sprint_id) 
      REFERENCES sprints (id) ON UPDATE NO ACTION ON DELETE CASCADE
);

CREATE INDEX idx_user_sprints_status ON user_sprints (user_id, status);

-- User Tasks
CREATE TABLE user_tasks (
   user_id integer NOT NULL,
   task_id integer NOT NULL,
   completed boolean DEFAULT false,
   completed_at timestamp without time zone,
   CONSTRAINT user_tasks_pkey PRIMARY KEY (user_id, task_id),
   CONSTRAINT user_tasks_user_id_fkey FOREIGN KEY (user_id) 
      REFERENCES users (id) ON UPDATE NO ACTION ON DELETE CASCADE,
   CONSTRAINT user_tasks_task_id_fkey FOREIGN KEY (task_id) 
      REFERENCES tasks (id) ON UPDATE NO ACTION ON DELETE CASCADE
);

CREATE INDEX idx_user_tasks_completed ON user_tasks (user_id, completed);

-- User Notes
CREATE TABLE user_notes (
   user_id integer NOT NULL,
   note_id integer NOT NULL,
   favorite boolean DEFAULT false,
   favorited_at timestamp without time zone,
   read boolean DEFAULT false,
   read_at timestamp without time zone,
   CONSTRAINT user_notes_pkey PRIMARY KEY (user_id, note_id),
   CONSTRAINT user_notes_user_id_fkey FOREIGN KEY (user_id) 
      REFERENCES users (id) ON UPDATE NO ACTION ON DELETE CASCADE,
   CONSTRAINT user_notes_note_id_fkey FOREIGN KEY (note_id) 
      REFERENCES notes (id) ON UPDATE NO ACTION ON DELETE CASCADE
);

-- User Achievements
CREATE TABLE user_achievements (
   user_id integer NOT NULL,
   achievement_id integer NOT NULL,
   progress integer DEFAULT 0,
   unlocked boolean DEFAULT false,
   unlocked_at timestamp without time zone,
   CONSTRAINT user_achievements_pkey PRIMARY KEY (user_id, achievement_id),
   CONSTRAINT user_achievements_user_id_fkey FOREIGN KEY (user_id) 
      REFERENCES users (id) ON UPDATE NO ACTION ON DELETE CASCADE,
   CONSTRAINT user_achievements_achievement_id_fkey FOREIGN KEY (achievement_id) 
      REFERENCES achievements (id) ON UPDATE NO ACTION ON DELETE CASCADE
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS user_achievements CASCADE;
DROP TABLE IF EXISTS user_notes CASCADE;
DROP TABLE IF EXISTS user_tasks CASCADE;
DROP TABLE IF EXISTS user_sprints CASCADE;
DROP TABLE IF EXISTS user_settings CASCADE;
-- +goose StatementEnd

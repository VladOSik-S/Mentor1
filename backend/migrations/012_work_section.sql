-- +goose Up
-- +goose StatementBegin

-- Настройки HH.ru для пользователя
CREATE TABLE user_hh_settings (
   user_id integer NOT NULL,
   access_token text,
   refresh_token text,
   token_expires_at timestamp without time zone,
   resume_id character varying(255), -- ID резюме на HH
   is_active boolean DEFAULT false,
   created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
   updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
   CONSTRAINT user_hh_settings_pkey PRIMARY KEY (user_id),
   CONSTRAINT user_hh_settings_user_id_fkey FOREIGN KEY (user_id) 
      REFERENCES users (id) ON UPDATE NO ACTION ON DELETE CASCADE
);

-- Сопроводительные письма (шаблоны)
CREATE SEQUENCE IF NOT EXISTS cover_letters_id_seq
   START WITH 1
   INCREMENT BY 1
   MINVALUE 1
   MAXVALUE 2147483647
   CACHE 1;

CREATE TABLE cover_letters (
   id integer DEFAULT nextval('cover_letters_id_seq'::regclass) NOT NULL,
   user_id integer NOT NULL,
   title character varying(255) NOT NULL,
   content text NOT NULL,
   is_default boolean DEFAULT false,
   created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
   updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
   CONSTRAINT cover_letters_pkey PRIMARY KEY (id),
   CONSTRAINT cover_letters_user_id_fkey FOREIGN KEY (user_id) 
      REFERENCES users (id) ON UPDATE NO ACTION ON DELETE CASCADE
);

CREATE INDEX idx_cover_letters_user ON cover_letters (user_id);

-- Статистика откликов
CREATE TABLE applications_stats (
   user_id integer NOT NULL,
   date date NOT NULL,
   total_applications integer DEFAULT 0,
   daily_limit integer DEFAULT 200, -- лимит HH.ru
   responses_received integer DEFAULT 0,
   interviews_scheduled integer DEFAULT 0,
   CONSTRAINT applications_stats_pkey PRIMARY KEY (user_id, date),
   CONSTRAINT applications_stats_user_id_fkey FOREIGN KEY (user_id) 
      REFERENCES users (id) ON UPDATE NO ACTION ON DELETE CASCADE
);

CREATE INDEX idx_applications_stats_date ON applications_stats (user_id, date DESC);

-- Лог действий по вакансиям
CREATE SEQUENCE IF NOT EXISTS work_activity_log_id_seq
   START WITH 1
   INCREMENT BY 1
   MINVALUE 1
   MAXVALUE 2147483647
   CACHE 1;

CREATE TABLE work_activity_log (
   id integer DEFAULT nextval('work_activity_log_id_seq'::regclass) NOT NULL,
   user_id integer NOT NULL,
   vacancy_id character varying(255), -- ID вакансии на HH
   vacancy_title character varying(500),
   company_name character varying(255),
   action_type character varying(50) NOT NULL, -- applied, hr_response, interview_scheduled, rejection, offer
   description text,
   created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
   CONSTRAINT work_activity_log_pkey PRIMARY KEY (id),
   CONSTRAINT work_activity_log_user_id_fkey FOREIGN KEY (user_id) 
      REFERENCES users (id) ON UPDATE NO ACTION ON DELETE CASCADE
);

CREATE INDEX idx_work_activity_log_user ON work_activity_log (user_id, created_at DESC);
CREATE INDEX idx_work_activity_log_vacancy ON work_activity_log (vacancy_id);

-- Отклики (для отслеживания)
CREATE TABLE job_applications (
   user_id integer NOT NULL,
   vacancy_id character varying(255) NOT NULL,
   vacancy_title character varying(500),
   company_name character varying(255),
   salary_from integer,
   salary_to integer,
   salary_currency character varying(10),
   vacancy_url text,
   cover_letter_id integer,
   status character varying(50) DEFAULT 'pending', -- pending, viewed, rejected, interview, offer
   applied_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
   last_status_update timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
   CONSTRAINT job_applications_pkey PRIMARY KEY (user_id, vacancy_id),
   CONSTRAINT job_applications_user_id_fkey FOREIGN KEY (user_id) 
      REFERENCES users (id) ON UPDATE NO ACTION ON DELETE CASCADE,
   CONSTRAINT job_applications_cover_letter_fkey FOREIGN KEY (cover_letter_id) 
      REFERENCES cover_letters (id) ON UPDATE NO ACTION ON DELETE SET NULL
);

CREATE INDEX idx_job_applications_status ON job_applications (user_id, status);
CREATE INDEX idx_job_applications_date ON job_applications (user_id, applied_at DESC);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS job_applications CASCADE;
DROP TABLE IF EXISTS work_activity_log CASCADE;
DROP SEQUENCE IF EXISTS work_activity_log_id_seq;
DROP TABLE IF EXISTS applications_stats CASCADE;
DROP TABLE IF EXISTS cover_letters CASCADE;
DROP SEQUENCE IF EXISTS cover_letters_id_seq;
DROP TABLE IF EXISTS user_hh_settings CASCADE;
-- +goose StatementEnd

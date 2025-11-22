-- +goose Up
-- +goose StatementBegin
CREATE SEQUENCE IF NOT EXISTS calendar_events_id_seq
   START WITH 1
   INCREMENT BY 1
   MINVALUE 1
   MAXVALUE 2147483647
   CACHE 1;

CREATE TABLE calendar_events (
   id integer DEFAULT nextval('calendar_events_id_seq'::regclass) NOT NULL,
   title character varying(255) NOT NULL,
   description text,
   event_type character varying(50) DEFAULT 'general'::character varying, -- general, lesson, meeting, deadline, holiday
   location text, -- URL или физическое место
   start_time timestamp without time zone NOT NULL,
   end_time timestamp without time zone NOT NULL,
   all_day boolean DEFAULT false,
   
   -- Повторения
   is_recurring boolean DEFAULT false,
   recurrence_rule character varying(255), -- DAILY, WEEKLY, MONTHLY, YEARLY
   recurrence_interval integer DEFAULT 1, -- каждые N дней/недель/месяцев
   recurrence_days integer[], -- дни недели для WEEKLY (0-6, где 0 - воскресенье)
   recurrence_end_date timestamp without time zone, -- когда заканчивается повторение
   
   -- Для кого доступно
   is_public boolean DEFAULT false, -- видят все
   target_role character varying(50), -- admin, student, guest - NULL значит для всех ролей
   created_by integer,
   
   created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
   updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
   
   CONSTRAINT calendar_events_pkey PRIMARY KEY (id),
   CONSTRAINT calendar_events_created_by_fkey FOREIGN KEY (created_by) 
      REFERENCES users (id) ON UPDATE NO ACTION ON DELETE SET NULL
);

CREATE INDEX idx_calendar_events_start_time ON calendar_events (start_time);
CREATE INDEX idx_calendar_events_type ON calendar_events (event_type);
CREATE INDEX idx_calendar_events_public ON calendar_events (is_public);

-- Пользовательские события (личные события)
CREATE TABLE user_calendar_events (
   user_id integer NOT NULL,
   event_id integer NOT NULL,
   is_hidden boolean DEFAULT false, -- скрыть глобальное событие
   reminder_minutes integer, -- за сколько минут напомнить
   notes text, -- личные заметки к событию
   
   CONSTRAINT user_calendar_events_pkey PRIMARY KEY (user_id, event_id),
   CONSTRAINT user_calendar_events_user_id_fkey FOREIGN KEY (user_id) 
      REFERENCES users (id) ON UPDATE NO ACTION ON DELETE CASCADE,
   CONSTRAINT user_calendar_events_event_id_fkey FOREIGN KEY (event_id) 
      REFERENCES calendar_events (id) ON UPDATE NO ACTION ON DELETE CASCADE
);

-- Личные события пользователей
CREATE SEQUENCE IF NOT EXISTS personal_events_id_seq
   START WITH 1
   INCREMENT BY 1
   MINVALUE 1
   MAXVALUE 2147483647
   CACHE 1;

CREATE TABLE personal_calendar_events (
   id integer DEFAULT nextval('personal_events_id_seq'::regclass) NOT NULL,
   user_id integer NOT NULL,
   title character varying(255) NOT NULL,
   description text,
   location text,
   start_time timestamp without time zone NOT NULL,
   end_time timestamp without time zone NOT NULL,
   all_day boolean DEFAULT false,
   
   -- Повторения (аналогично calendar_events)
   is_recurring boolean DEFAULT false,
   recurrence_rule character varying(255),
   recurrence_interval integer DEFAULT 1,
   recurrence_days integer[],
   recurrence_end_date timestamp without time zone,
   
   reminder_minutes integer,
   color character varying(20), -- для визуального отличия
   
   created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
   updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
   
   CONSTRAINT personal_calendar_events_pkey PRIMARY KEY (id),
   CONSTRAINT personal_calendar_events_user_id_fkey FOREIGN KEY (user_id) 
      REFERENCES users (id) ON UPDATE NO ACTION ON DELETE CASCADE
);

CREATE INDEX idx_personal_events_user ON personal_calendar_events (user_id);
CREATE INDEX idx_personal_events_start ON personal_calendar_events (user_id, start_time);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS personal_calendar_events CASCADE;
DROP SEQUENCE IF EXISTS personal_events_id_seq;
DROP TABLE IF EXISTS user_calendar_events CASCADE;
DROP TABLE IF EXISTS calendar_events CASCADE;
DROP SEQUENCE IF EXISTS calendar_events_id_seq;
-- +goose StatementEnd

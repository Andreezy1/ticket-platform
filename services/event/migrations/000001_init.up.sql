CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE FUNCTION fn_set_updated_at() RETURNS trigger AS $$
BEGIN
    NEW.updated_at := now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql
   SET search_path = pg_catalog, public;

-- часовой пояс проверяется триггером, а не CHECK: CHECK-констрейнт не
-- может делать subquery к таблице/view, а pg_timezone_names — view
CREATE FUNCTION fn_validate_venue_timezone() RETURNS trigger AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_timezone_names WHERE name = NEW.timezone
    ) THEN
        RAISE EXCEPTION 'invalid IANA timezone: %', NEW.timezone;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql
   SET search_path = pg_catalog, public;

-- =========================================================
-- venues — площадки
-- Без мягкого удаления: на id площадки никто извне не ссылается,
-- физический DELETE безопасен (и заблокирован RESTRICT-ом от events,
-- пока у площадки есть мероприятия).
-- =========================================================
CREATE TABLE venues (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name        varchar(255) NOT NULL,
    address     text NOT NULL,
    timezone    varchar(64) NOT NULL,      -- IANA tz, например "Europe/Riga"
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TRIGGER trg_venues_updated_at
    BEFORE UPDATE ON venues
    FOR EACH ROW EXECUTE FUNCTION fn_set_updated_at();

CREATE TRIGGER trg_venues_validate_timezone
    BEFORE INSERT OR UPDATE OF timezone ON venues
    FOR EACH ROW EXECUTE FUNCTION fn_validate_venue_timezone();

-- =========================================================
-- events — мероприятия
-- Мягкое удаление оставлено: event_id — внешний идентификатор,
-- на который ссылается Booking Service (без FK, через границу сервиса).
-- Физическое удаление осиротило бы чужие брони. status (cancelled/
-- archived) — бизнес-состояние, deleted_at — техническое удаление;
-- это разные оси, не путать.
-- =========================================================
CREATE TABLE events (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    venue_id      uuid NOT NULL REFERENCES venues(id) ON DELETE RESTRICT,
    title         varchar(255) NOT NULL,
    description   text NULL,
    status        varchar(32) NOT NULL DEFAULT 'draft',
    starts_at     timestamptz NOT NULL,
    ends_at       timestamptz NULL,
    published_at  timestamptz NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    deleted_at    timestamptz NULL,

    CONSTRAINT chk_events_description_not_empty
        CHECK (description IS NULL OR description <> ''),

    -- varchar+CHECK вместо ENUM: список статусов будет расти
    -- (postponed, archived, ...), а убрать значение из ENUM в PG нельзя
    CONSTRAINT chk_events_status
        CHECK (status IN ('draft', 'published', 'postponed', 'cancelled', 'archived')),

    CONSTRAINT chk_events_ends_after_starts
        CHECK (ends_at IS NULL OR ends_at > starts_at),

    -- одностороння импликация, не эквивалентность: published_at обязателен
    -- в момент публикации, но НЕ обнуляется constraint'ом при последующей
    -- отмене/переносе — это исторический факт "когда стало видно покупателям"
    CONSTRAINT chk_events_published_at
        CHECK (status <> 'published' OR published_at IS NOT NULL)
);

-- листинг опубликованных + keyset-пагинация по дате
CREATE INDEX idx_events_published_starts_at
    ON events (starts_at, id)
    WHERE status = 'published' AND deleted_at IS NULL;

CREATE INDEX idx_events_venue_id
    ON events (venue_id)
    WHERE deleted_at IS NULL;

CREATE TRIGGER trg_events_updated_at
    BEFORE UPDATE ON events
    FOR EACH ROW EXECUTE FUNCTION fn_set_updated_at();

-- нельзя добавить/перепривязать зону к мягко удалённому мероприятию —
-- FK этого не видит, deleted_at для него просто значение в столбце
CREATE FUNCTION fn_check_event_not_deleted() RETURNS trigger AS $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM events
         WHERE id = NEW.event_id AND deleted_at IS NOT NULL
    ) THEN
        RAISE EXCEPTION 'event % is deleted, cannot attach zone to it', NEW.event_id;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql
   SET search_path = pg_catalog, public;

-- =========================================================
-- event_zones — ценовые зоны мероприятия
-- Без мягкого удаления: живут только внутри события, физически
-- каскадируются вместе с ним. capacity — просто число: без
-- event_seats это единственный и неделимый источник вместимости.
-- =========================================================
CREATE TABLE event_zones (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id     uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    name         varchar(255) NOT NULL,
    capacity     integer NOT NULL CHECK (capacity >= 0),
    price_cents  bigint NOT NULL CHECK (price_cents >= 0),
    currency     varchar(3) NOT NULL DEFAULT 'EUR',
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT uq_event_zones_event_name UNIQUE (event_id, name),

    -- регистр + формат ISO 4217 (буквы), а не только длина 3 символа —
    -- иначе 'eur' и 'EUR' считаются разными значениями
    CONSTRAINT chk_event_zones_currency_format
        CHECK (currency ~ '^[A-Z]{3}$')
);

CREATE INDEX idx_event_zones_event_id
    ON event_zones (event_id);

CREATE TRIGGER trg_event_zones_updated_at
    BEFORE UPDATE ON event_zones
    FOR EACH ROW EXECUTE FUNCTION fn_set_updated_at();

CREATE TRIGGER trg_event_zones_check_event_active
    BEFORE INSERT OR UPDATE OF event_id ON event_zones
    FOR EACH ROW EXECUTE FUNCTION fn_check_event_not_deleted();
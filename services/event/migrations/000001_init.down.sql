DROP TABLE IF EXISTS event_zones;
DROP TABLE IF EXISTS events;
DROP TABLE IF EXISTS venues;

DROP FUNCTION IF EXISTS fn_check_event_not_deleted();
DROP FUNCTION IF EXISTS fn_validate_venue_timezone();
DROP FUNCTION IF EXISTS fn_set_updated_at();


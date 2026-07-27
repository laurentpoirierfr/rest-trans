-- ==========================================
-- SSE NOTIFY TRIGGERS
-- ==========================================

-- Fonction générique de notification pour SSE
CREATE OR REPLACE FUNCTION rest_notify() RETURNS trigger AS $$
DECLARE
    payload jsonb;
    channel text;
BEGIN
    channel := TG_TABLE_SCHEMA || '_' || TG_TABLE_NAME;

    IF TG_OP = 'INSERT' THEN
        payload := jsonb_build_object(
            'schema', TG_TABLE_SCHEMA,
            'table', TG_TABLE_NAME,
            'op', TG_OP,
            'new', to_jsonb(NEW)
        );
    ELSIF TG_OP = 'UPDATE' THEN
        payload := jsonb_build_object(
            'schema', TG_TABLE_SCHEMA,
            'table', TG_TABLE_NAME,
            'op', TG_OP,
            'old', to_jsonb(OLD),
            'new', to_jsonb(NEW)
        );
    ELSIF TG_OP = 'DELETE' THEN
        payload := jsonb_build_object(
            'schema', TG_TABLE_SCHEMA,
            'table', TG_TABLE_NAME,
            'op', TG_OP,
            'old', to_jsonb(OLD)
        );
    END IF;

    PERFORM pg_notify('rest_' || channel, payload::text);
    RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

-- Triggers sur les tables de démo
CREATE TRIGGER users_notify
    AFTER INSERT OR UPDATE OR DELETE ON users
    FOR EACH ROW EXECUTE FUNCTION rest_notify();

CREATE TRIGGER projects_notify
    AFTER INSERT OR UPDATE OR DELETE ON projects
    FOR EACH ROW EXECUTE FUNCTION rest_notify();

CREATE TRIGGER project_tasks_notify
    AFTER INSERT OR UPDATE OR DELETE ON project_tasks
    FOR EACH ROW EXECUTE FUNCTION rest_notify();

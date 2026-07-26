-- ==========================================
-- FONCTIONS DE TEST POUR RPC
-- ==========================================

-- 1. Fonction simple : obtenir le profil d'un utilisateur
CREATE OR REPLACE FUNCTION get_user_profile(p_user_id INT)
RETURNS TABLE (
    user_id INT,
    user_name VARCHAR,
    user_email VARCHAR,
    project_count BIGINT,
    task_count BIGINT
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        u.id,
        u.name,
        u.email,
        (SELECT count(*) FROM projects p WHERE p.author_id = u.id),
        (SELECT count(*) FROM project_tasks pt 
         JOIN projects p ON pt.project_id = p.id 
         WHERE p.author_id = u.id)
    FROM users u
    WHERE u.id = p_user_id;
END;
$$ LANGUAGE plpgsql;


-- 2. Fonction avec paramètres nommés : rechercher des projets
CREATE OR REPLACE FUNCTION search_projects(
    p_search TEXT DEFAULT '',
    p_status VARCHAR DEFAULT NULL,
    p_limit INT DEFAULT 10,
    p_offset INT DEFAULT 0
)
RETURNS TABLE (
    project_id INT,
    project_title VARCHAR,
    project_status VARCHAR,
    author_name VARCHAR,
    task_count BIGINT
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        pr.id,
        pr.title,
        pr.status,
        COALESCE(u.name, 'No author'),
        (SELECT count(*) FROM project_tasks pt WHERE pt.project_id = pr.id)
    FROM projects pr
    LEFT JOIN users u ON pr.author_id = u.id
    WHERE (p_search = '' OR pr.title ILIKE '%' || p_search || '%')
      AND (p_status IS NULL OR pr.status = p_status)
    ORDER BY pr.id
    LIMIT p_limit
    OFFSET p_offset;
END;
$$ LANGUAGE plpgsql;


-- 3. Fonction retourne un seul scalar : compter les éléments
CREATE OR REPLACE FUNCTION get_stats()
RETURNS TABLE (
    total_users BIGINT,
    total_projects BIGINT,
    total_tasks BIGINT,
    public_projects BIGINT
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        (SELECT count(*) FROM users),
        (SELECT count(*) FROM projects),
        (SELECT count(*) FROM project_tasks),
        (SELECT count(*) FROM projects WHERE (settings->>'is_public')::boolean = true);
END;
$$ LANGUAGE plpgsql;


-- 4. Fonction INSERT via RPC : créer un projet avec tâches
CREATE OR REPLACE FUNCTION create_project_with_tasks(
    p_title VARCHAR,
    p_author_id INT,
    p_tasks JSONB DEFAULT '[]'::jsonb
)
RETURNS TABLE (
    project_id INT,
    project_title VARCHAR,
    tasks_created INT
) AS $$
DECLARE
    new_project_id INT;
    task JSONB;
    task_count INT := 0;
BEGIN
    INSERT INTO projects (title, author_id, status)
    VALUES (p_title, p_author_id, 'draft')
    RETURNING id INTO new_project_id;

    FOR task IN SELECT * FROM jsonb_array_elements(p_tasks)
    LOOP
        INSERT INTO project_tasks (project_id, task_order, title, payload)
        VALUES (
            new_project_id,
            COALESCE((task->>'task_order')::int, task_count + 1),
            task->>'title',
            COALESCE(task->'payload', '{"priority":"medium","tags":[]}'::jsonb)
        );
        task_count := task_count + 1;
    END LOOP;

    RETURN QUERY
    SELECT new_project_id, p_title, task_count;
END;
$$ LANGUAGE plpgsql;


-- 5. Fonction avec INOUT : bump le priority d'une tâche
CREATE OR REPLACE FUNCTION bump_task_priority(p_task_id INT)
RETURNS TABLE (
    task_id INT,
    old_priority TEXT,
    new_priority TEXT
) AS $$
DECLARE
    current_priority TEXT;
    bumped TEXT;
BEGIN
    SELECT payload->>'priority' INTO current_priority
    FROM project_tasks WHERE id = p_task_id;

    bumped := CASE current_priority
        WHEN 'low' THEN 'medium'
        WHEN 'medium' THEN 'high'
        WHEN 'high' THEN 'critical'
        ELSE 'medium'
    END;

    UPDATE project_tasks
    SET payload = jsonb_set(payload, '{priority}', to_jsonb(bumped))
    WHERE id = p_task_id;

    RETURN QUERY
    SELECT p_task_id, current_priority, bumped;
END;
$$ LANGUAGE plpgsql;

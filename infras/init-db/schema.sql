-- Extension pour la génération automatique de UUID (si besoin de clés UUID)
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ==========================================
-- 1. ENTITÉ INDÉPENDANTE (Auteur / Client)
-- ==========================================
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    email VARCHAR(150) UNIQUE NOT NULL,
    profile_metadata JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

COMMENT ON TABLE users IS '@allow GET,HEAD';

-- ==========================================
-- 2. EN-TÊTE / AGRÉGATION (Projet)
-- Agrégation : Un projet est associé à un utilisateur (auteur), 
-- mais si l'utilisateur est supprimé, le projet peut subsister (ON DELETE SET NULL).
-- ==========================================
CREATE TABLE projects (
    id SERIAL PRIMARY KEY,
    title VARCHAR(200) NOT NULL,
    author_id INT, -- Clé étrangère d'agrégation
    status VARCHAR(50) DEFAULT 'draft',
    settings JSONB DEFAULT '{
        "is_public": false,
        "features_enabled": [],
        "limits": {"max_tasks": 100}
    }'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,

    -- Clé étrangère pour l'Agrégation (relation optionnelle / faible)
    CONSTRAINT fk_project_author 
        FOREIGN KEY (author_id) 
        REFERENCES users(id) 
        ON DELETE SET NULL
);

-- ==========================================
-- 3. COMPOSITION (Tâches du projet)
-- Composition : Une tâche fait partie intégrante d'un projet. 
-- Si le projet parent est supprimé, ses tâches sont automatiquement supprimées (ON DELETE CASCADE).
-- ==========================================
CREATE TABLE project_tasks (
    id SERIAL PRIMARY KEY,
    project_id INT NOT NULL, -- Clé étrangère de composition (NOT NULL)
    task_order INT NOT NULL,
    title VARCHAR(200) NOT NULL,
    payload JSONB NOT NULL DEFAULT '{
        "priority": "medium",
        "tags": [],
        "custom_fields": {}
    }'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,

    -- Clé étrangère pour la Composition (relation forte avec cascade)
    CONSTRAINT fk_task_project 
        FOREIGN KEY (project_id) 
        REFERENCES projects(id) 
        ON DELETE CASCADE,

    -- Clé d'unicité composite (exemple) : ordre unique au sein d'un même projet
    CONSTRAINT uq_project_task_order 
        UNIQUE (project_id, task_order)
);

-- ==========================================
-- INDEXATION JSONB (Indispensable pour les études de perf)
-- ==========================================

-- Index GIN global sur le payload des tâches (recherche sur n'importe quel champ JSON)
CREATE INDEX idx_tasks_payload_gin ON project_tasks USING gin (payload);

-- Index ciblé (B-Tree) sur la valeur "priority" extraite du JSONB
CREATE INDEX idx_tasks_priority ON project_tasks ((payload->>'priority'));

-- Index GIN spécifique avec jsonb_path_ops pour des recherches très rapides de type conteneur (@>)
CREATE INDEX idx_projects_settings_gin ON projects USING gin (settings jsonb_path_ops);

-- ==========================================
-- TRANSACTIONS (Saga pattern)
-- ==========================================
CREATE TABLE rest_transactions (
    id VARCHAR(36) PRIMARY KEY,
    schema_name VARCHAR(255) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE TABLE rest_transaction_operations (
    id SERIAL PRIMARY KEY,
    transaction_id VARCHAR(36) NOT NULL REFERENCES rest_transactions(id) ON DELETE CASCADE,
    operation VARCHAR(10) NOT NULL,
    table_name TEXT NOT NULL,
    sql_query TEXT NOT NULL,
    params JSONB,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_rest_tx_ops_txid ON rest_transaction_operations(transaction_id);
CREATE INDEX idx_rest_tx_status ON rest_transactions(status);

-- ==========================================
-- VUES (pour tests de read-only)
-- ==========================================
CREATE VIEW active_users AS
SELECT id, name, email, created_at
FROM users
WHERE email NOT LIKE '%deleted%';

COMMENT ON TABLE active_users IS 'Vue des utilisateurs actifs (read-only)';

-- ==========================================
-- FULL-TEXT SEARCH (pour tests FTS)
-- ==========================================
CREATE TABLE articles (
    id SERIAL PRIMARY KEY,
    title TEXT NOT NULL,
    body tsvector NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_articles_body ON articles USING gin(body);

COMMENT ON TABLE articles IS '@fts_language english';
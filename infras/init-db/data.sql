-- Nettoyage si réexécution
TRUNCATE TABLE users, projects, project_tasks RESTART IDENTITY CASCADE;

-- 1. Insertion des Utilisateurs
INSERT INTO users (name, email, profile_metadata) VALUES
('Alice Dupont', 'alice@example.com', '{"role": "admin", "preferences": {"theme": "dark", "notifications": true}}'),
('Bob Martin', 'bob@example.com', '{"role": "editor", "preferences": {"theme": "light"}}'),
('Charlie Durand', 'charlie@example.com', '{"role": "viewer"}');

-- 2. Insertion des Projets (Agrégation avec users)
INSERT INTO projects (title, author_id, status, settings) VALUES
(
    'Refonte Site E-Commerce', 
    1, -- Projet créé par Alice
    'active', 
    '{"is_public": true, "features_enabled": ["cart", "stripe", "reviews"], "limits": {"max_tasks": 500}}'
),
(
    'Migration Base de Données', 
    2, -- Projet créé par Bob
    'in_progress', 
    '{"is_public": false, "features_enabled": ["backup", "alerts"], "limits": {"max_tasks": 50}}'
),
(
    'Projet Orphelin Test', 
    NULL, -- Agrégation optionnelle : pas d'auteur attribué
    'draft', 
    '{"is_public": false, "features_enabled": []}'
);

-- 3. Insertion des Tâches (Composition avec projects)
INSERT INTO project_tasks (project_id, task_order, title, payload) VALUES
-- Tâches du Projet 1 (E-Commerce)
(
    1, 
    1, 
    'Maquettage UI', 
    '{"priority": "high", "tags": ["design", "front"], "estimated_hours": 12, "custom_fields": {"assignee": "Alice", "sprint": 1}}'
),
(
    1, 
    2, 
    'Integration Stripe', 
    '{"priority": "critical", "tags": ["backend", "payment"], "estimated_hours": 24, "custom_fields": {"assignee": "Bob", "sprint": 2}}'
),

-- Tâches du Projet 2 (Migration DB)
(
    2, 
    1, 
    'Audit des schémas existants', 
    '{"priority": "medium", "tags": ["database", "audit"], "estimated_hours": 8, "custom_fields": {"blocking_issue": false}}'
),
(
    2, 
    2, 
    'Ecriture des scripts SQL', 
    '{"priority": "high", "tags": ["database", "sql"], "estimated_hours": 16, "custom_fields": {"reviewers": ["Alice", "Charlie"]}}'
);
-- Database initialization script for PostgreSQL

-- Create database if not exists (run as superuser)
-- CREATE DATABASE sandtracker;

-- Connect to the database
-- \c sandtracker

-- Create app_users table
CREATE TABLE IF NOT EXISTS app_users (
    id UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    username VARCHAR(50) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    display_name VARCHAR(100),
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    last_login TIMESTAMP WITH TIME ZONE
);

-- Create sand_lists table
CREATE TABLE IF NOT EXISTS sand_lists (
    id UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    rop VARCHAR(100) NOT NULL,
    date DATE NOT NULL,
    work_type VARCHAR(20) NOT NULL CHECK (work_type IN ('офис', 'удаленка')),
    names TEXT NOT NULL,
    checkboxes JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create user_sessions table for session management
CREATE TABLE IF NOT EXISTS user_sessions (
    id UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES app_users(id) ON DELETE CASCADE,
    token VARCHAR(255) UNIQUE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL
);

-- Create employee_credentials table for storing login, password and internal number
CREATE TABLE IF NOT EXISTS employee_credentials (
    id UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    sand_list_id UUID NOT NULL REFERENCES sand_lists(id) ON DELETE CASCADE,
    employee_name VARCHAR(255) NOT NULL,
    login VARCHAR(100),
    password VARCHAR(255),
    internal_number VARCHAR(50),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(sand_list_id, employee_name)
);

-- Create indexes
CREATE INDEX IF NOT EXISTS idx_app_users_username ON app_users(username);
CREATE INDEX IF NOT EXISTS idx_app_users_is_active ON app_users(is_active);
CREATE INDEX IF NOT EXISTS idx_sand_lists_created_at ON sand_lists(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_user_sessions_token ON user_sessions(token);
CREATE INDEX IF NOT EXISTS idx_user_sessions_expires_at ON user_sessions(expires_at);
CREATE INDEX IF NOT EXISTS idx_employee_credentials_sand_list_id ON employee_credentials(sand_list_id);
CREATE INDEX IF NOT EXISTS idx_employee_credentials_employee_name ON employee_credentials(employee_name);

-- Insert default users (passwords are hashed with SHA256)
-- Stalker: 16084636
INSERT INTO app_users (username, password_hash, display_name) VALUES 
    ('Stalker', encode(sha256('16084636'::bytea), 'hex'), 'Stalker')
ON CONFLICT (username) DO NOTHING;

-- Bob: z53Z2OsJ1
INSERT INTO app_users (username, password_hash, display_name) VALUES 
    ('Bob', encode(sha256('z53Z2OsJ1'::bytea), 'hex'), 'Bob')
ON CONFLICT (username) DO NOTHING;

-- Apple: z53Z2OsJ2
INSERT INTO app_users (username, password_hash, display_name) VALUES 
    ('Apple', encode(sha256('z53Z2OsJ2'::bytea), 'hex'), 'Apple')
ON CONFLICT (username) DO NOTHING;

-- Admin: z53Z2OsJ67
INSERT INTO app_users (username, password_hash, display_name) VALUES 
    ('Admin', encode(sha256('z53Z2OsJ67'::bytea), 'hex'), 'Admin')
ON CONFLICT (username) DO NOTHING;

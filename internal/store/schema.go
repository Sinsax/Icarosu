package store

func allSchemaStatements() []string {
	return []string{
		`CREATE TABLE IF NOT EXISTS members (
			group_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			nickname TEXT NOT NULL DEFAULT '',
			card TEXT NOT NULL DEFAULT '',
			relationship TEXT NOT NULL DEFAULT '',
			note TEXT NOT NULL DEFAULT '',
			message_count INTEGER NOT NULL DEFAULT 0,
			first_seen_at DATETIME NOT NULL,
			last_seen_at DATETIME NOT NULL,
			PRIMARY KEY (group_id, user_id)
		)`,
		`CREATE TABLE IF NOT EXISTS memories (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			group_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL DEFAULT 0,
			content TEXT NOT NULL,
			created_at DATETIME NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			group_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			sender_name TEXT NOT NULL DEFAULT '',
			content TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'user',
			created_at DATETIME NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			group_id INTEGER NOT NULL,
			title TEXT NOT NULL,
			content TEXT NOT NULL,
			created_at DATETIME NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS event_participants (
			event_id INTEGER NOT NULL,
			group_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			role TEXT NOT NULL DEFAULT 'participant',
			PRIMARY KEY (event_id, user_id)
		)`,
		`CREATE TABLE IF NOT EXISTS message_replies (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			group_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			message_id INTEGER NOT NULL,
			reply_to_message_id INTEGER NOT NULL,
			created_at DATETIME NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS recalls (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			group_id INTEGER NOT NULL,
			message_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			operator_id INTEGER NOT NULL,
			created_at DATETIME NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS user_activity (
			group_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			hour INTEGER NOT NULL,
			weekday INTEGER NOT NULL,
			message_count INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (group_id, user_id, hour, weekday)
		)`,
		`CREATE TABLE IF NOT EXISTS interactions (
			group_id INTEGER NOT NULL,
			from_user_id INTEGER NOT NULL,
			to_user_id INTEGER NOT NULL,
			count INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (group_id, from_user_id, to_user_id)
		)`,
	}
}

func v2SchemaStatements() []string {
	return []string{
		`CREATE TABLE IF NOT EXISTS memory_embeddings (
			memory_id INTEGER PRIMARY KEY,
			group_id INTEGER NOT NULL,
			vector TEXT NOT NULL,
			model TEXT NOT NULL DEFAULT 'local',
			created_at DATETIME NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_memory_embeddings_group ON memory_embeddings(group_id)`,
	}
}

func v3SchemaStatements() []string {
	return []string{
		`CREATE TABLE IF NOT EXISTS topic_states (
			group_id INTEGER PRIMARY KEY,
			topic TEXT NOT NULL DEFAULT '',
			last_user_msg TEXT NOT NULL DEFAULT '',
			last_reply TEXT NOT NULL DEFAULT '',
			stability INTEGER NOT NULL DEFAULT 0,
			vector TEXT NOT NULL DEFAULT '',
			updated_at DATETIME NOT NULL
		)`,
	}
}

func v4SchemaStatements() []string {
	return []string{
		`CREATE TABLE IF NOT EXISTS style_expressions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			group_id INTEGER NOT NULL,
			tags TEXT NOT NULL,
			expression TEXT NOT NULL,
			created_at DATETIME NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_style_expressions_group ON style_expressions(group_id)`,
		`CREATE TABLE IF NOT EXISTS jargons (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			group_id INTEGER NOT NULL,
			term TEXT NOT NULL,
			meaning TEXT NOT NULL DEFAULT '',
			source_user_id INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_jargons_group ON jargons(group_id)`,
		`CREATE TABLE IF NOT EXISTS user_profiles (
			group_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			nickname TEXT NOT NULL DEFAULT '',
			interests TEXT NOT NULL DEFAULT '',
			message_count INTEGER NOT NULL DEFAULT 0,
			last_seen_at DATETIME NOT NULL,
			PRIMARY KEY (group_id, user_id)
		)`,
	}
}

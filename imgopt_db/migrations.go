package imgopt_db

var migrations []string = []string{
	// uploaders
	`
		CREATE TABLE IF NOT EXISTS "uploaders" (
			"id"	INTEGER NOT NULL,
			"uuid"	TEXT,
			PRIMARY KEY("id")
		);
	`,

	// projects
	`
		CREATE TABLE IF NOT EXISTS "projects" (
			"id"	INTEGER NOT NULL,
			"uploader_id"	INTEGER,
			"title"	TEXT,
			"created_at"	TEXT,
			"updated_at"	TEXT,
			PRIMARY KEY("id"),
			FOREIGN KEY("uploader_id") REFERENCES "uploaders"("id") ON UPDATE CASCADE ON DELETE CASCADE
		);
	`,

	// optimizations
	`
		CREATE TABLE IF NOT EXISTS "optimizations" (
			"id"	INTEGER NOT NULL,
			"project_id"	INTEGER,
			"output_extension"	TEXT,
			"output_size_percent"	INTEGER,
			"created_at"	TEXT,
			"updated_at"	TEXT,
			PRIMARY KEY("id"),
			FOREIGN KEY("project_id") REFERENCES "projects"("id") ON UPDATE CASCADE ON DELETE CASCADE
		);
	`,

	// folders
	`
		CREATE TABLE IF NOT EXISTS "folders" (
			"id"	INTEGER NOT NULL,
			"project_id"	INTEGER,
			"optimization_id"	INTEGER,
			"path"	TEXT,
			"created_at"	TEXT,
			"updated_at"	TEXT,
			PRIMARY KEY("id"),
			FOREIGN KEY("optimization_id") REFERENCES "optimizations"("id") ON UPDATE CASCADE ON DELETE CASCADE,
			FOREIGN KEY("project_id") REFERENCES "projects"("id") ON UPDATE CASCADE ON DELETE CASCADE
		);
	`,

	// images
	`
		CREATE TABLE IF NOT EXISTS "images" (
			"id"	INTEGER NOT NULL,
			"folder_id"	INTEGER,
			"url" TEXT,
			"extension"	TEXT,
			"filename"	TEXT,
			"size_bytes"	INTEGER,
			"width"	INTEGER,
			"height"	INTEGER,
			"created_at"	TEXT,
			"updated_at"	TEXT,
			PRIMARY KEY("id"),
			FOREIGN KEY("folder_id") REFERENCES "folders"("id") ON UPDATE CASCADE ON DELETE CASCADE
		);
	`,
}

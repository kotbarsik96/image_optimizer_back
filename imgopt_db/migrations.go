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

	// images
	`
		CREATE TABLE IF NOT EXISTS "images" (
			"id"	INTEGER NOT NULL,
			"extension"	TEXT,
			"filename"	TEXT,
			"path"	TEXT,
			"size_bytes"	INTEGER,
			"width"	INTEGER,
			"height"	INTEGER,
			"created_at"	TEXT,
			"updated_at"	TEXT,
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

	// projects_images
	`
		CREATE TABLE IF NOT EXISTS "projects_images" (
			"project_id"	INTEGER,
			"image_id"	INTEGER,
			FOREIGN KEY("project_id") REFERENCES "projects"("id") ON UPDATE CASCADE ON DELETE CASCADE
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

	// optimized_images
	`
		CREATE TABLE IF NOT EXISTS "optimized_images" (
			"id"	INTEGER NOT NULL,
			"optimization_id"	INTEGER,
			"image_id"	INTEGER,
			"original_image_id"	INTEGER,
			"created_at"	TEXT,
			"updated_at"	TEXT,
			PRIMARY KEY("id"),
			FOREIGN KEY("image_id") REFERENCES "images"("id") ON UPDATE CASCADE ON DELETE CASCADE,
			FOREIGN KEY("optimization_id") REFERENCES "optimizations"("id") ON UPDATE CASCADE ON DELETE CASCADE,
			FOREIGN KEY("original_image_id") REFERENCES "images"("id") ON UPDATE CASCADE ON DELETE CASCADE
		);
	`,
}

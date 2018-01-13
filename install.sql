-- front-end tables
CREATE TABLE checks (
	id INT NOT NULL PRIMARY KEY AUTO_INCREMENT,
	name VARCHAR(64) NOT NULL,
	type VARCHAR(16) NOT NULL,
	data VARCHAR(1024) NOT NULL,
	check_interval INT NOT NULL DEFAULT 60,
	delay INT NOT NULL DEFAULT 1,
	status ENUM ('online', 'offline') DEFAULT 'online',
	user_id INT NOT NULL,
	enabled TINYINT(1) NOT NULL DEFAULT 1
);

CREATE TABLE contacts (
	id INT NOT NULL PRIMARY KEY AUTO_INCREMENT,
	type VARCHAR(32) NOT NULL,
	data VARCHAR(512) NOT NULL,
	user_id INT NOT NULL
);

CREATE TABLE alerts (
	id INT NOT NULL PRIMARY KEY AUTO_INCREMENT,
	check_id INT NOT NULL,
	contact_id INT NOT NULL,
	type ENUM ('online', 'offline', 'both') DEFAULT 'both',
	user_id INT NOT NULL,
	enabled TINYINT(1) NOT NULL DEFAULT 1
);

-- data tables
CREATE TABLE check_events (
	id INT NOT NULL PRIMARY KEY AUTO_INCREMENT,
	check_id INT NOT NULL,
	type ENUM ('online', 'offline'),
	time TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE charges (
	id INT NOT NULL PRIMARY KEY AUTO_INCREMENT,
	check_id INT NOT NULL,
	type VARCHAR(32) NOT NULL,
	data VARCHAR(512) NOT NULL
);

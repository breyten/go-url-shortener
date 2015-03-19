# Copied from Mathias Bynens' PHP URL Shortener: https://github.com/mathiasbynens/php-url-shortener

# DROP TABLE IF EXISTS `redirect`;

# Why you should use `utf8mb4` instead of `utf8`: http://mathiasbynens.be/notes/mysql-utf8mb4
CREATE TABLE `redirect` (
	`slug` varchar(14) collate utf8mb4_unicode_ci NOT NULL,
	`url` varchar(620) collate utf8mb4_unicode_ci NOT NULL,
	`date` datetime NOT NULL,
	`hits` bigint(20) NOT NULL default '0',
	PRIMARY KEY (`slug`),
	KEY(`url`(191))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Used for the URL shortener';

INSERT INTO `redirect` VALUES ('g', 'https://github.com/samwierema/go-url-shortener', NOW(), 1);

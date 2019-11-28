CREATE DATABASE IF NOT EXISTS MSXT;
use msxt;

CREATE TABLE Users (
    id int PRIMARY KEY AUTO_INCREMENT NOT NULL,
    username varchar(20),
    password varchar(32),
    kind int(1)
);

CREATE TABLE Coupons (
    id int PRIMARY KEY AUTO_INCREMENT NOT NULL,
    username varchar(20),
    coupons varchar(60),
    amount int,
    `left` int,
    description varchar(60)
);
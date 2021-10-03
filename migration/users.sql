create table users
(
    id serial not null
        constraint users_pk
            primary key,
    login varchar(32),
    password varchar(64),
    photo text
);


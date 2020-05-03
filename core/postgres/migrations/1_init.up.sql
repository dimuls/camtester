create table task (
    id bigserial primary key,
    type text not null,
    payload jsonb not null
);

create table task_result (
    id bigserial primary key,
    task_id bigint not null references task (id) unique,
    ok bool not null,
    payload jsonb not null
);

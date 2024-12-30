-- name: NewBar :exec
INSERT INTO foo (bar)
VALUES (?);

-- name: GetBar :many
SELECT
    id
    ,bar
FROM foo;

-- +goose Up
CREATE TABLE chirp(
    id UUID PRIMARY KEY, 
    created_at TIMESTAMP NOT NULL, 
    updated_at TIMESTAMP NOT NULL, 
    body TEXT NOT NULL,
    user_id UUID NOT NULL ,
    CONSTRAINT user_context
    FOREIGN KEY (user_id)
    REFERENCES users(id)
    ON DELETE CASCADE
    );


-- +goose Down
DROP TABLE chirp;


CREATE TABLE IF NOT EXISTS loxa_events
(
    ts DateTime64(3) DEFAULT now64(3),
    raw String
)
ENGINE = MergeTree
ORDER BY ts;

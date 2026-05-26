DELETE FROM connections c1
USING connections c2
WHERE c1.requester_id = c2.addressee_id
AND c1.addressee_id = c2.requester_id
AND c1.created_at > c2.created_at;

CREATE UNIQUE INDEX idx_unique_connection_pair
ON connections (LEAST(requester_id, addressee_id), GREATEST(requester_id, addressee_id));

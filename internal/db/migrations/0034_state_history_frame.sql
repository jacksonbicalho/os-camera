-- thumbnail da transicao de estado (path servivel sob /recordings, vazio = sem frame)
ALTER TABLE camera_state_history ADD COLUMN frame_path TEXT NOT NULL DEFAULT '';

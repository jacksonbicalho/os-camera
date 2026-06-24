package config

import "strconv"

// Entries devolve os campos operacionais do bootstrap como pares {chave-pontuada, valor},
// em ordem fixa, para o comando `camera config`. NÃO expõe a senha do admin nem o valor
// do jwt_secret — para o JWT mostra apenas o modo (gerado a cada boot vs fixo).
func (c Config) Entries() [][2]string {
	jwt := "(gerado a cada boot)"
	if c.Server.JWTSecret != "" {
		jwt = "(fixo)"
	}
	return [][2]string{
		{"server.port", strconv.Itoa(c.Server.Port)},
		{"timezone", c.Timezone},
		{"debug", strconv.FormatBool(c.Debug)},
		{"db_path", c.DBPath},
		{"storage.path", c.Storage.Path},
		{"server.segments_path", c.Server.SegmentsPath},
		{"log.output", c.Log.Output},
		{"admin.username", c.Admin.Username},
		{"server.jwt_secret", jwt},
	}
}

// EntryKeys devolve só as chaves válidas (na ordem), para mensagens de ajuda/erro.
func (c Config) EntryKeys() []string {
	e := c.Entries()
	keys := make([]string, len(e))
	for i, kv := range e {
		keys[i] = kv[0]
	}
	return keys
}

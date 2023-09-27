package linkpearl

import (
	"strings"

	"maunium.net/go/mautrix/id"
)

// GetAccountData of the user (from cache and API, with encryption support)
func (l *Linkpearl) GetAccountData(name string) (map[string]string, error) {
	cached, ok := l.acc.Get(name)
	if ok {
		if cached == nil {
			return map[string]string{}, nil
		}
		return cached, nil
	}

	var data map[string]string
	err := l.GetClient().GetAccountData(name, &data)
	if err != nil {
		data = map[string]string{}
		if strings.Contains(err.Error(), "M_NOT_FOUND") {
			l.acc.Add(name, data)
			return data, nil
		}
		return data, UnwrapError(err)
	}
	data = l.decryptAccountData(data)

	l.acc.Add(name, data)
	return data, err
}

// SetAccountData of the user (to cache and API, with encryption support)
func (l *Linkpearl) SetAccountData(name string, data map[string]string) error {
	l.acc.Add(name, data)

	data = l.encryptAccountData(data)
	return UnwrapError(l.GetClient().SetAccountData(name, data))
}

// GetRoomAccountData of the room (from cache and API, with encryption support)
func (l *Linkpearl) GetRoomAccountData(roomID id.RoomID, name string) (map[string]string, error) {
	key := roomID.String() + name
	cached, ok := l.acc.Get(key)
	if ok {
		if cached == nil {
			return map[string]string{}, nil
		}
		return cached, nil
	}

	var data map[string]string
	err := l.GetClient().GetRoomAccountData(roomID, name, &data)
	if err != nil {
		data = map[string]string{}
		if strings.Contains(err.Error(), "M_NOT_FOUND") {
			l.acc.Add(key, data)
			return data, nil
		}
		return data, UnwrapError(err)
	}
	data = l.decryptAccountData(data)

	l.acc.Add(key, data)
	return data, err
}

// SetRoomAccountData of the room (to cache and API, with encryption support)
func (l *Linkpearl) SetRoomAccountData(roomID id.RoomID, name string, data map[string]string) error {
	key := roomID.String() + name
	l.acc.Add(key, data)

	data = l.encryptAccountData(data)
	return UnwrapError(l.GetClient().SetRoomAccountData(roomID, name, data))
}

func (l *Linkpearl) encryptAccountData(data map[string]string) map[string]string {
	if l.acr == nil {
		return data
	}

	encrypted := make(map[string]string, len(data))
	for k, v := range data {
		ek, err := l.acr.Encrypt(k)
		if err != nil {
			l.log.Error().Err(err).Str("key", k).Msg("cannot encrypt account data")
		}
		ev, err := l.acr.Encrypt(v)
		if err != nil {
			l.log.Error().Err(err).Str("key", k).Msg("cannot encrypt account data")
		}
		encrypted[ek] = ev // worst case: plaintext value
	}

	return encrypted
}

func (l *Linkpearl) decryptAccountData(data map[string]string) map[string]string {
	if l.acr == nil {
		return data
	}

	decrypted := make(map[string]string, len(data))
	for ek, ev := range data {
		k, err := l.acr.Decrypt(ek)
		if err != nil {
			l.log.Error().Err(err).Str("key", k).Msg("cannot decrypt account data")
		}
		v, err := l.acr.Decrypt(ev)
		if err != nil {
			l.log.Error().Err(err).Str("key", k).Msg("cannot decrypt account data")
		}
		decrypted[k] = v // worst case: encrypted value, usual case: migration from plaintext to encrypted account data
	}

	return decrypted
}

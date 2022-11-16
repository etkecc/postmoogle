package linkpearl

import (
	"strings"

	"maunium.net/go/mautrix/id"
)

// GetAccountData of the user (from cache and API, with encryption support)
func (l *Linkpearl) GetAccountData(name string) (map[string]string, error) {
	cached, ok := l.acc.Get(name)
	if ok {
		l.logAccountData(l.log.Debug, "GetAccountData(%q) cached:", cached, name)
		if cached == nil {
			return map[string]string{}, nil
		}
		return cached, nil
	}

	var data map[string]string
	err := l.GetClient().GetAccountData(name, &data)
	if err != nil {
		l.logAccountData(l.log.Debug, "GetAccountData(%q) error: %v", nil, name, err)
		data = map[string]string{}
		if strings.Contains(err.Error(), "M_NOT_FOUND") {
			l.acc.Add(name, data)
			return data, nil
		}
		return data, err
	}
	data = l.decryptAccountData(data)
	l.logAccountData(l.log.Debug, "GetAccountData(%q):", data, name)

	l.acc.Add(name, data)
	return data, err
}

// SetAccountData of the user (to cache and API, with encryption support)
func (l *Linkpearl) SetAccountData(name string, data map[string]string) error {
	l.acc.Add(name, data)

	l.logAccountData(l.log.Debug, "SetAccountData(%q):", data, name)
	data = l.encryptAccountData(data)
	return l.GetClient().SetAccountData(name, data)
}

// GetRoomAccountData of the room (from cache and API, with encryption support)
func (l *Linkpearl) GetRoomAccountData(roomID id.RoomID, name string) (map[string]string, error) {
	key := roomID.String() + name
	cached, ok := l.acc.Get(key)
	if ok {
		l.logAccountData(l.log.Debug, "GetRoomAccountData(%q, %q) cached:", cached, roomID, name)
		if cached == nil {
			return map[string]string{}, nil
		}
		return cached, nil
	}

	var data map[string]string
	err := l.GetClient().GetRoomAccountData(roomID, name, &data)
	if err != nil {
		l.logAccountData(l.log.Debug, "GetRoomAccountData(%q, %q) error: %v", nil, roomID, name, err)
		data = map[string]string{}
		if strings.Contains(err.Error(), "M_NOT_FOUND") {
			l.acc.Add(key, data)
			return data, nil
		}
		return data, err
	}
	data = l.decryptAccountData(data)
	l.logAccountData(l.log.Debug, "GetRoomAccountData(%q, %q):", data, roomID, name)

	l.acc.Add(key, data)
	return data, err
}

// SetRoomAccountData of the room (to cache and API, with encryption support)
func (l *Linkpearl) SetRoomAccountData(roomID id.RoomID, name string, data map[string]string) error {
	key := roomID.String() + name
	l.acc.Add(key, data)

	l.logAccountData(l.log.Debug, "SetRoomAccountData(%q, %q):", data, roomID, name)
	data = l.encryptAccountData(data)
	return l.GetClient().SetRoomAccountData(roomID, name, data)
}

func (l *Linkpearl) encryptAccountData(data map[string]string) map[string]string {
	if l.acr == nil {
		return data
	}

	encrypted := make(map[string]string, len(data))
	for k, v := range data {
		ek, err := l.acr.Encrypt(k)
		if err != nil {
			l.log.Error("cannot encrypt account data (key=%q): %v", k, err)
		}
		ev, err := l.acr.Encrypt(v)
		if err != nil {
			l.log.Error("cannot encrypt account data (key=%q): %v", k, err)
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
			l.log.Error("cannot decrypt account data (key=%q): %v", k, err)
		}
		v, err := l.acr.Decrypt(ev)
		if err != nil {
			l.log.Error("cannot decrypt account data (key=%q): %v", k, err)
		}
		decrypted[k] = v // worst case: encrypted value, usual case: migration from plaintext to encrypted account data
	}

	return decrypted
}

func (l *Linkpearl) logAccountData(method func(string, ...any), message string, data map[string]string, args ...any) {
	if len(data) == 0 {
		method(message, args...)
		return
	}

	safeData := make(map[string]string, len(data))
	for k, v := range data {
		sv, ok := l.aclr[k]
		if ok {
			safeData[k] = sv
			continue
		}

		safeData[k] = v
	}
	args = append(args, safeData)

	method(message+" %+v", args...)
}

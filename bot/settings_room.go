package bot

import (
	"strings"

	"maunium.net/go/mautrix/id"

	"gitlab.com/etke.cc/postmoogle/utils"
)

// account data key
const acRoomSettingsKey = "cc.etke.postmoogle.settings"

// option keys
const (
	roomOptionOwner     = "owner"
	roomOptionMailbox   = "mailbox"
	roomOptionNoSend    = "nosend"
	roomOptionNoSender  = "nosender"
	roomOptionNoSubject = "nosubject"
	roomOptionNoHTML    = "nohtml"
	roomOptionNoThreads = "nothreads"
	roomOptionNoFiles   = "nofiles"
)

type roomSettings map[string]string

// Get option
func (s roomSettings) Get(key string) string {
	return s[strings.ToLower(strings.TrimSpace(key))]
}

// Set option
func (s roomSettings) Set(key, value string) {
	s[strings.ToLower(strings.TrimSpace(key))] = value
}

func (s roomSettings) Mailbox() string {
	return s.Get(roomOptionMailbox)
}

func (s roomSettings) Owner() string {
	return s.Get(roomOptionOwner)
}

func (s roomSettings) NoSend() bool {
	return utils.Bool(s.Get(roomOptionNoSend))
}

func (s roomSettings) NoSender() bool {
	return utils.Bool(s.Get(roomOptionNoSender))
}

func (s roomSettings) NoSubject() bool {
	return utils.Bool(s.Get(roomOptionNoSubject))
}

func (s roomSettings) NoHTML() bool {
	return utils.Bool(s.Get(roomOptionNoHTML))
}

func (s roomSettings) NoThreads() bool {
	return utils.Bool(s.Get(roomOptionNoThreads))
}

func (s roomSettings) NoFiles() bool {
	return utils.Bool(s.Get(roomOptionNoFiles))
}

// ContentOptions converts room display settings to content options
func (s roomSettings) ContentOptions() *utils.ContentOptions {
	return &utils.ContentOptions{
		HTML:    !s.NoHTML(),
		Sender:  !s.NoSender(),
		Subject: !s.NoSubject(),
		Threads: !s.NoThreads(),

		FromKey:      eventFromKey,
		SubjectKey:   eventSubjectKey,
		MessageIDKey: eventMessageIDkey,
		InReplyToKey: eventInReplyToKey,
	}
}

func (b *Bot) getRoomSettings(roomID id.RoomID) (roomSettings, error) {
	cfg := b.cfg.Get(roomID.String())
	if cfg != nil {
		return cfg, nil
	}

	config := roomSettings{}
	err := b.lp.GetClient().GetRoomAccountData(roomID, acRoomSettingsKey, &config)
	if err != nil {
		if strings.Contains(err.Error(), "M_NOT_FOUND") {
			// Suppress `M_NOT_FOUND (HTTP 404): Room account data not found` errors.
			// Until some settings are explicitly set, we don't store any.
			// In such cases, just return a default (empty) settings object.
			err = nil
		}
	}

	if err == nil {
		b.cfg.Set(roomID.String(), config)
	}

	return config, utils.UnwrapError(err)
}

func (b *Bot) setRoomSettings(roomID id.RoomID, cfg roomSettings) error {
	b.cfg.Set(roomID.String(), cfg)
	return utils.UnwrapError(b.lp.GetClient().SetRoomAccountData(roomID, acRoomSettingsKey, cfg))
}

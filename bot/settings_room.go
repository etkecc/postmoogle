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
	roomOptionOwner          = "owner"
	roomOptionMailbox        = "mailbox"
	roomOptionNoSend         = "nosend"
	roomOptionNoSender       = "nosender"
	roomOptionNoRecipient    = "norecipient"
	roomOptionNoSubject      = "nosubject"
	roomOptionNoHTML         = "nohtml"
	roomOptionNoThreads      = "nothreads"
	roomOptionNoFiles        = "nofiles"
	roomOptionPassword       = "password"
	roomOptionSecuritySMTP   = "security:smtp"
	roomOptionSecurityEmail  = "security:email"
	roomOptionSpamEmails     = "spam:emails"
	roomOptionSpamHosts      = "spam:hosts"
	roomOptionSpamLocalparts = "spam:localparts"
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

func (s roomSettings) Password() string {
	return s.Get(roomOptionPassword)
}

func (s roomSettings) NoSend() bool {
	return utils.Bool(s.Get(roomOptionNoSend))
}

func (s roomSettings) NoSender() bool {
	return utils.Bool(s.Get(roomOptionNoSender))
}

func (s roomSettings) NoRecipient() bool {
	return utils.Bool(s.Get(roomOptionNoRecipient))
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

func (s roomSettings) SecuritySMTP() bool {
	return utils.Bool(s.Get(roomOptionSecuritySMTP))
}

func (s roomSettings) SecurityEmail() bool {
	return utils.Bool(s.Get(roomOptionSecurityEmail))
}

func (s roomSettings) SpamEmails() []string {
	return utils.StringSlice(s.Get(roomOptionSpamEmails))
}

func (s roomSettings) SpamHosts() []string {
	return utils.StringSlice(s.Get(roomOptionSpamHosts))
}

func (s roomSettings) SpamLocalparts() []string {
	return utils.StringSlice(s.Get(roomOptionSpamLocalparts))
}

// ContentOptions converts room display settings to content options
func (s roomSettings) ContentOptions() *utils.ContentOptions {
	return &utils.ContentOptions{
		HTML:      !s.NoHTML(),
		Sender:    !s.NoSender(),
		Recipient: !s.NoRecipient(),
		Subject:   !s.NoSubject(),
		Threads:   !s.NoThreads(),

		FromKey:      eventFromKey,
		SubjectKey:   eventSubjectKey,
		MessageIDKey: eventMessageIDkey,
		InReplyToKey: eventInReplyToKey,
	}
}

func (b *Bot) getRoomSettings(roomID id.RoomID) (roomSettings, error) {
	config, err := b.lp.GetRoomAccountData(roomID, acRoomSettingsKey)
	return config, utils.UnwrapError(err)
}

func (b *Bot) setRoomSettings(roomID id.RoomID, cfg roomSettings) error {
	return utils.UnwrapError(b.lp.SetRoomAccountData(roomID, acRoomSettingsKey, cfg))
}

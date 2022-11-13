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
	roomOptionOwner         = "owner"
	roomOptionMailbox       = "mailbox"
	roomOptionNoSend        = "nosend"
	roomOptionNoSender      = "nosender"
	roomOptionNoRecipient   = "norecipient"
	roomOptionNoSubject     = "nosubject"
	roomOptionNoHTML        = "nohtml"
	roomOptionNoThreads     = "nothreads"
	roomOptionNoFiles       = "nofiles"
	roomOptionPassword      = "password"
	roomOptionSpamcheckSMTP = "spamcheck:smtp"
	roomOptionSpamcheckMX   = "spamcheck:mx"
	roomOptionSpamlist      = "spamlist"
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

func (s roomSettings) SpamcheckSMTP() bool {
	return utils.Bool(s.Get(roomOptionSpamcheckSMTP))
}

func (s roomSettings) SpamcheckMX() bool {
	return utils.Bool(s.Get(roomOptionSpamcheckMX))
}

func (s roomSettings) Spamlist() []string {
	return utils.StringSlice(s.Get(roomOptionSpamlist))
}

func (s roomSettings) migrateSpamlistSettings() {
	uniq := map[string]struct{}{}
	emails := utils.StringSlice(s.Get("spamlist:emails"))
	localparts := utils.StringSlice(s.Get("spamlist:localparts"))
	hosts := utils.StringSlice(s.Get("spamlist:hosts"))
	list := utils.StringSlice(s.Get(roomOptionSpamlist))
	delete(s, "spamlist:emails")
	delete(s, "spamlist:localparts")
	delete(s, "spamlist:hosts")

	for _, email := range emails {
		if email == "" {
			continue
		}
		uniq[email] = struct{}{}
	}

	for _, localpart := range localparts {
		if localpart == "" {
			continue
		}
		uniq[localpart+"@*"] = struct{}{}
	}

	for _, host := range hosts {
		if host == "" {
			continue
		}
		uniq["*@"+host] = struct{}{}
	}

	for _, item := range list {
		if item == "" {
			continue
		}
		uniq[item] = struct{}{}
	}

	spamlist := make([]string, 0, len(uniq))
	for item := range uniq {
		spamlist = append(spamlist, item)
	}
	s.Set(roomOptionSpamlist, strings.Join(spamlist, ","))
}

// ContentOptions converts room display settings to content options
func (s roomSettings) ContentOptions() *utils.ContentOptions {
	return &utils.ContentOptions{
		HTML:      !s.NoHTML(),
		Sender:    !s.NoSender(),
		Recipient: !s.NoRecipient(),
		Subject:   !s.NoSubject(),
		Threads:   !s.NoThreads(),

		ToKey:         eventToKey,
		FromKey:       eventFromKey,
		SubjectKey:    eventSubjectKey,
		MessageIDKey:  eventMessageIDkey,
		InReplyToKey:  eventInReplyToKey,
		ReferencesKey: eventReferencesKey,
	}
}

func (b *Bot) getRoomSettings(roomID id.RoomID) (roomSettings, error) {
	config, err := b.lp.GetRoomAccountData(roomID, acRoomSettingsKey)
	if config == nil {
		config = map[string]string{}
	}

	return config, utils.UnwrapError(err)
}

func (b *Bot) setRoomSettings(roomID id.RoomID, cfg roomSettings) error {
	return utils.UnwrapError(b.lp.SetRoomAccountData(roomID, acRoomSettingsKey, cfg))
}

func (b *Bot) migrateRoomSettings(roomID id.RoomID) {
	cfg, err := b.getRoomSettings(roomID)
	if err != nil {
		b.log.Error("cannot retrieve room settings: %v", err)
		return
	}

	if cfg["spamlist:emails"] == "" && cfg["spamlist:localparts"] == "" && cfg["spamlist:hosts"] == "" {
		return
	}
	cfg.migrateSpamlistSettings()
	err = b.setRoomSettings(roomID, cfg)
	if err != nil {
		b.log.Error("cannot migrate room settings: %v", err)
	}
}
